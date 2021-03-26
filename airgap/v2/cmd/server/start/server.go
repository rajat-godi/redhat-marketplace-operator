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

package server

import (
	"context"
	"fmt"
	"io"

	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/fileserver"
	v1 "github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/model/v1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/pkg/database"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	fileserver.UnimplementedFileServerServer
	Log  logr.Logger
	File database.File
}

func (s *Server) UploadFile(stream fileserver.FileServer_UploadFileServer) error {
	var bs []byte
	var finfo *v1.FileInfo
	var fid *v1.FileID
	var size uint32

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			s.Log.Info(fmt.Sprintf("Stream end, total bytes received: %v", len(bs)))
			// Attempt to save file in database
			err := s.File.SaveFile(finfo, bs)
			if err != nil {
				return status.Errorf(
					codes.Unknown,
					fmt.Sprintf("Failed to save file in database: %v", err),
				)
			}
			// Prepare response on save and close stream
			res := &fileserver.UploadFileResponse{
				FileId: fid,
				Size:   size,
			}
			return stream.SendAndClose(res)
		} else if err != nil {
			s.Log.Error(err, "Oops, something went wrong!")
			return status.Errorf(
				codes.Unknown,
				fmt.Sprintf("Error while processing stream, details: %v", err),
			)
		}

		b := req.GetChunkData()
		if b != nil && bs == nil {
			bs = b
		} else if b != nil {
			bs = append(bs, b...)
		} else if req.GetInfo() != nil {
			finfo = req.GetInfo()
			fid = finfo.GetFileId()
			size = finfo.GetSize()
		}
	}
}

func (s *Server) ListFileMetadata(lis *fileserver.ListFileMetadataRequest, stream fileserver.FileServer_ListFileMetadataServer) error {
	return status.Errorf(
		codes.Unimplemented,
		"Method Unimplemented",
	)
}

func (s *Server) GetFileMetadata(ctx context.Context, gfmr *fileserver.GetFileMetadataRequest) (*fileserver.GetFileMetadataResponse, error) {
	return nil, status.Errorf(
		codes.Unimplemented,
		"Method Unimplemented",
	)
}

func (s *Server) DownloadFile(dfr *fileserver.DownloadFileRequest, stream fileserver.FileServer_DownloadFileServer) error {
	return status.Errorf(
		codes.Unimplemented,
		"Method Unimplemented",
	)
}

func (s *Server) UpdateFileMetadata(ctx context.Context, ufmr *fileserver.UpdateFileMetadataRequest) (*fileserver.UpdateFileMetadataResponse, error) {
	return nil, status.Errorf(
		codes.Unimplemented,
		"Method Unimplemented",
	)
}

func (s *Server) DeleteFile(ctx context.Context, dfr *fileserver.DeleteFileRequest) (*fileserver.DeleteFileResponse, error) {
	return nil, status.Errorf(
		codes.Unimplemented,
		"Method Unimplemented",
	)
}

func (s *Server) CleanTombstones(ctx context.Context, ctr *fileserver.CleanTombstonesRequest) (*fileserver.CleanTombstonesResponse, error) {
	return nil, status.Errorf(
		codes.Unimplemented,
		"Method Unimplemented",
	)
}
