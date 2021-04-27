// Copyright 2021 IBM Corp.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package list

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/fileretreiver"
	v1 "github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/model/v1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/cmd/client/util"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

type Listconfig struct {
	filter    []string
	sort      []string
	outputDir string
	outputCSV bool
	conn      *grpc.ClientConn
	client    fileretreiver.FileRetreiverClient
}

var (
	lc        Listconfig
	log       logr.Logger
	file_name = "files.csv"
)

var ListCmd = &cobra.Command{
	Use:   "list",
	Short: "Fetch list of file ",
	Long: `Fetch list of files

Filter flags used for filtering list to be fetched using pre-defined 
keys or custom key and sort flag used for sorting list based on sort key and sort order.

    Allowed filter operators: EQUAL, CONTAINS, LESS_THAN, GREATER_THAN
    Allowed sort operators: ASC, DESC
    -----------------------------------------------------------------------
    Pre-defined filter keys: 
    [provided_id] refers to the file identifier
    [provided_name] refers to the name of the file
    [size] refers to the size of file
    [created_at] refers to file creation date (expected format yyyy-mm-dd or RFC3339)
    [deleted_at] refers to the file deletion date  (expected format yyyy-mm-dd or RFC3339)
    -----------------------------------------------------------------------
    Pre-defined sort keys: 
    [provided_id] refers to the file identifier
    [provided_name] refers to the name of the file
    [size] refers to the size of file
    [created_at] refers to file creation date (expected format yyyy-mm-dd or RFC3339)
    [deleted_at] refers to the file deletion date  (expected format yyyy-mm-dd or RFC3339)
	`,
	Example: `
    # List all latest files.
    client list 
	
    # List all files between specific dates.
    client list --filter="created_at GREATER_THAN 2020-12-25" --filter="created_at LESS_THAN 2021-03-30"
	
    # List files having specific metadata
    client list --filter="description CONTAINS 'operator file'"
	
    # List files uploaded after specific dates and sort it by file name in ascending order
    client list --filter="created_at GREATER_THAN 2021-03-20" --sort="provided_name ASC"
	
    # Sort list by size in ascending order
    client list -sort="size ASC"
	
    # Save list to csv file
    client list  --output-dir=/path/to/dir`,
	RunE: func(cmd *cobra.Command, args []string) error {

		l, err := util.InitLog()
		if err != nil {
			return err
		}
		log = l
		// Initialize client
		err = lc.initializeListClient()
		if err != nil {
			return err
		}
		if cmd.Flag("output-dir").Changed {
			lc.outputCSV = true
		}
		defer lc.closeConnection()
		return lc.listFileMetadata()
	},
}

func init() {
	ListCmd.Flags().StringSliceVarP(&lc.filter, "filter", "f", []string{}, "filter file list based on pre-defined or custom keys")
	ListCmd.Flags().StringSliceVarP(&lc.sort, "sort", "s", []string{}, "sort file list based key and sort operation used")
	ListCmd.Flags().StringVarP(&lc.outputDir, "output-dir", "o", "", "path to save list")
}

func (lc *Listconfig) initializeListClient() error {
	conn, err := util.InitClient()
	if err != nil {
		return err
	}
	lc.client = fileretreiver.NewFileRetreiverClient(conn)
	lc.conn = conn
	return nil
}

// closeConnection closes the grpc client connection
func (lc *Listconfig) closeConnection() {
	if lc != nil && lc.conn != nil {
		lc.conn.Close()
	}
}

// listFileMetadata fetch list of files and its metadata from the grpc server to a specified directory
func (lc *Listconfig) listFileMetadata() error {

	var filter_list []*fileretreiver.ListFileMetadataRequest_ListFileFilter
	var sort_list []*fileretreiver.ListFileMetadataRequest_ListFileSort
	var file *os.File
	var w *csv.Writer

	filter_list, err := parseFilter(lc.filter)
	if err != nil {
		return err
	}

	sort_list, err = parseSort(lc.sort)
	if err != nil {
		return err
	}

	req := &fileretreiver.ListFileMetadataRequest{
		FilterBy: filter_list,
		SortBy:   sort_list,
	}

	resultStream, err := lc.client.ListFileMetadata(context.Background(), req)
	if err != nil {
		return fmt.Errorf("failed to retrieve list due to: %v", err)
	}

	fp := lc.outputDir + string(os.PathSeparator) + file_name
	if lc.outputCSV {
		file, err = os.Create(fp)
		if err != nil {
			return err
		}
		w = csv.NewWriter(file)
		defer file.Close()
		defer w.Flush()
		err = writeToCSV(getHeaders(), w)
		if err != nil {
			return err
		}
	} else {
		printList(getHeaders())
	}
	for {
		response, err := resultStream.Recv()
		if err == io.EOF {
			if lc.outputCSV {
				log.Info("List stored", "location:", fp)
			}
			break
		} else if err != nil {
			if lc.outputCSV {
				defer os.Remove(fp)
			}
			return fmt.Errorf("error while reading stream: %v", err)
		}

		data := response.GetResults()
		if lc.outputCSV {
			row, err := parseFinfo(data)
			if err != nil {
				defer os.Remove(fp)
				return err
			}
			err = writeToCSV(row, w)
			if err != nil {
				defer os.Remove(fp)
				return err
			}
		} else {
			row, err := parseFinfo(data)
			if err != nil {
				return err
			}
			err = printList(row)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// parseFilter parses arguments for filter operation
// It returns list of struct ListFileMetadataRequest_ListFileFilter and error if occured
func parseFilter(filter []string) ([]*fileretreiver.ListFileMetadataRequest_ListFileFilter, error) {

	modelColumnSet := map[string]bool{
		"provided_name": true,
		"provided_id":   true,
		"size":          true,
		"created_at":    true,
		"deleted_at":    true,
	}

	var list_filter []*fileretreiver.ListFileMetadataRequest_ListFileFilter
	for _, filter_string := range filter {

		filter_args := parseArgs(filter_string)

		if len(filter_args) != 3 {
			return nil,
				fmt.Errorf("'%v' : invalid number of arguments provided for filter operation, Required 3 | Provided %v ",
					filter_string, len(filter_args))
		}

		operator, err := parseFilterOperator(filter_args[1], modelColumnSet[filter_args[0]])
		if err != nil {
			return nil, err
		}

		var value string
		if filter_args[0] == "created_at" || filter_args[0] == "deleted_at" {
			value, err = parseDateToEpoch(filter_args[2])
			if err != nil {
				return nil, err
			}
		} else {
			value = filter_args[2]
		}

		req := &fileretreiver.ListFileMetadataRequest_ListFileFilter{
			Key:      filter_args[0],
			Operator: *operator,
			Value:    value,
		}
		list_filter = append(list_filter, req)
	}
	return list_filter, nil
}

// parseFilterOperator parses filter operator and returns respective operator defined by fileretreiver.proto
func parseFilterOperator(op string, isColumn bool) (*fileretreiver.ListFileMetadataRequest_ListFileFilter_Comparison, error) {
	var operator fileretreiver.ListFileMetadataRequest_ListFileFilter_Comparison
	if isColumn {
		switch op {
		case "EQUAL":
			operator = fileretreiver.ListFileMetadataRequest_ListFileFilter_EQUAL
		case "LESS_THAN":
			operator = fileretreiver.ListFileMetadataRequest_ListFileFilter_LESS_THAN
		case "GREATER_THAN":
			operator = fileretreiver.ListFileMetadataRequest_ListFileFilter_GREATER_THAN
		case "CONTAINS":
			operator = fileretreiver.ListFileMetadataRequest_ListFileFilter_CONTAINS
		default:
			return nil, fmt.Errorf("invalid filter operation used: %v ", op)
		}
	} else {
		switch op {
		case "EQUAL":
			operator = fileretreiver.ListFileMetadataRequest_ListFileFilter_EQUAL
		case "CONTAINS":
			operator = fileretreiver.ListFileMetadataRequest_ListFileFilter_CONTAINS
		default:
			return nil, fmt.Errorf("invalid filter operation used: %v ", op)
		}
	}

	return &operator, nil
}

// parseSort parses arguments for sort operation
// It returns list of struct ListFileMetadataRequest_ListFileSort and error id occured
func parseSort(sort_list []string) ([]*fileretreiver.ListFileMetadataRequest_ListFileSort, error) {
	modelColumnSet := map[string]bool{
		"provided_name": true,
		"provided_id":   true,
		"size":          true,
		"created_at":    true,
		"deleted_at":    true,
	}
	var list_sort []*fileretreiver.ListFileMetadataRequest_ListFileSort

	for _, sort_string := range sort_list {

		sort_args := parseArgs(sort_string)

		if len(sort_args) != 2 {
			return nil, fmt.Errorf("'%v' : invalid number of arguments provided for sort operation, Required 2 | Provided %v ", sort_string, len(sort_args))
		}
		if !modelColumnSet[sort_args[0]] {
			return nil, fmt.Errorf("invalid operand passed for sort operation: %v ", sort_args[0])
		}
		operator, err := parseSortOperator(sort_args[1])
		if err != nil {
			return nil, err
		}
		req := &fileretreiver.ListFileMetadataRequest_ListFileSort{
			Key:       sort_args[0],
			SortOrder: *operator,
		}
		list_sort = append(list_sort, req)
	}
	return list_sort, nil
}

// parseSortOperator parses sort operator and returns respective operator defined by fileretreiver.proto
func parseSortOperator(op string) (*fileretreiver.ListFileMetadataRequest_ListFileSort_SortOrder, error) {
	var operator fileretreiver.ListFileMetadataRequest_ListFileSort_SortOrder

	switch op {
	case "ASC":
		operator = fileretreiver.ListFileMetadataRequest_ListFileSort_ASC
	case "DESC":
		operator = fileretreiver.ListFileMetadataRequest_ListFileSort_DESC
	default:
		return nil, fmt.Errorf("invalid sort operation used: %v", op)
	}

	return &operator, nil
}

// parseDateToEpoch parses date argument
// It returns unix epoch of date entered and error if occured
// Date can be in RFC3339 format or yyyy-mm-dd format
// for any valid date in yyyy-mm-dd format time selected will be midnight in UTC
// Eg: Input date: 2021-04-13
//     Complete date-time in RFC3339 :  2021-04-13 00:00:00 +0000 UTC
//     Unix Epoch: 1618272000
func parseDateToEpoch(date string) (string, error) {

	const layout = "2006-01-02"
	dt, er1 := time.Parse(time.RFC3339, date)
	if er1 != nil {
		dt1, er2 := time.Parse(layout, date)
		if er2 != nil {
			return "", fmt.Errorf(er1.Error() + er2.Error())
		}
		dt = dt1
	}

	time_ := time.Date(
		dt.Year(),
		time.Month(dt.Month()),
		dt.Day(),
		dt.Hour(),
		dt.Minute(),
		dt.Second(),
		dt.Nanosecond(),
		time.UTC)
	return strconv.FormatInt(time_.Unix(), 10), nil
}

// parseArgs takes double quoted string as input and return slice of string
// group of characters between two consecutive spaces will be considered as string
// any characters between single quotation marks will be grouped as string
func parseArgs(arg_string string) []string {
	var args []string
	var bstr []byte
	isQuoted := false
	for i, c := range arg_string {
		if string(c) == "'" {
			isQuoted = !isQuoted
			st := strings.TrimSpace(string(bstr))
			if len(st) != 0 {
				args = append(args, st)
			}
			bstr = []byte{}
			continue
		}
		if isQuoted {
			bstr = append(bstr, byte(c))
			if i == (len(arg_string) - 1) {
				st := strings.TrimSpace(string(bstr))
				if len(st) != 0 {
					args = append(args, st)
				}
			}
		} else {
			if string(c) == " " {
				st := strings.TrimSpace(string(bstr))
				if len(st) != 0 {
					args = append(args, st)
				}
				bstr = []byte{}
				continue
			}
			bstr = append(bstr, byte(c))
			if i == (len(arg_string) - 1) {
				st := strings.TrimSpace(string(bstr))
				if len(st) != 0 {
					args = append(args, st)
				}
			}
		}
	}
	return args
}

//parses v1.Finfo and return slice of string
func parseFinfo(finfo *v1.FileInfo) ([]string, error) {
	mdata, err := json.Marshal(finfo.Metadata)
	if err != nil {
		return nil, err
	}
	metadata := string(mdata)
	return []string{
		finfo.FileId.GetId(),
		finfo.FileId.GetName(),
		strconv.FormatUint(uint64(finfo.GetSize()), 10),
		time.Unix(finfo.CreatedAt.Seconds, 0).Format(time.RFC3339),
		strconv.FormatBool(finfo.Compression),
		finfo.CompressionType,
		metadata,
	}, nil
}

// write stream output to csv
func writeToCSV(row []string, writer *csv.Writer) error {
	err := writer.Write(row)
	if err != nil {
		return err
	}
	return nil
}

// returns headers for csv
func getHeaders() []string {
	return []string{
		"File ID",
		"File Name",
		"Size",
		"Created At",
		"Compression",
		"Compression Type",
		"Metadata",
	}
}

// printList prints list of files on console
func printList(r []string) error {
	w := tabwriter.NewWriter(os.Stdout, 10, 0, 2, ' ', tabwriter.Debug)
	var row string
	for _, s := range r {
		row = row + s + "\t"
	}
	fmt.Fprintln(w, row)
	w.Flush()
	return nil
}
