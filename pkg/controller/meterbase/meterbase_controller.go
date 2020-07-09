// Copyright 2020 IBM Corp.
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

package meterbase

import (
	"context"
	"reflect"

	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/manifests"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"

	merrors "emperror.dev/errors"
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	status "github.com/operator-framework/operator-sdk/pkg/status"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	DEFAULT_PROM_SERVER            = "prom/prometheus:v2.15.2"
	DEFAULT_CONFIGMAP_RELOAD       = "jimmidyson/configmap-reload:v0.3.0"
	RELATED_IMAGE_PROM_SERVER      = "RELATED_IMAGE_PROM_SERVER"
	RELATED_IMAGE_CONFIGMAP_RELOAD = "RELATED_IMAGE_CONFIGMAP_RELOAD"
)

//ConfigmapReload: "jimmidyson/configmap-reload:v0.3.0",
//Server:          "prom/prometheus:v2.15.2",

var (
	log = logf.Log.WithName("controller_meterbase")

	meterbaseFlagSet *pflag.FlagSet
)

func init() {
	meterbaseFlagSet = pflag.NewFlagSet("meterbase", pflag.ExitOnError)
	meterbaseFlagSet.String("related-image-prom-server",
		utils.Getenv(RELATED_IMAGE_PROM_SERVER, DEFAULT_PROM_SERVER),
		"image for prometheus")
	meterbaseFlagSet.String("related-image-configmap-reload",
		utils.Getenv(RELATED_IMAGE_CONFIGMAP_RELOAD, DEFAULT_CONFIGMAP_RELOAD),
		"image for prometheus")
}

func FlagSet() *pflag.FlagSet {
	return meterbaseFlagSet
}

// Add creates a new MeterBase Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(
	mgr manager.Manager,
	ccprovider ClientCommandRunnerProvider,
) error {
	reconciler := newReconciler(mgr, ccprovider)
	return add(mgr, reconciler)
}

// blank assignment to verify that ReconcileMeterBase implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileMeterBase{}

// ReconcileMeterBase reconciles a MeterBase object
type ReconcileMeterBase struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client       client.Client
	scheme       *runtime.Scheme
	opts         *MeterbaseOpts
	ccprovider   ClientCommandRunnerProvider
	patchChecker *PatchChecker
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(
	mgr manager.Manager,
	ccprovider ClientCommandRunnerProvider) reconcile.Reconciler {
	promOpts := &MeterbaseOpts{
		PullPolicy: "IfNotPresent",
		AssetPath:  viper.GetString("assets"),
	}
	return &ReconcileMeterBase{
		client:       mgr.GetClient(),
		scheme:       mgr.GetScheme(),
		ccprovider:   ccprovider,
		patchChecker: NewPatchChecker(utils.RhmPatchMaker),
		opts:         promOpts,
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("meterbase-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource MeterBase
	err = c.Watch(&source.Kind{Type: &marketplacev1alpha1.MeterBase{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// watch configmap
	err = c.Watch(&source.Kind{Type: &corev1.ConfigMap{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &marketplacev1alpha1.MeterBase{},
	})
	if err != nil {
		return err
	}

	// watch prometheus
	err = c.Watch(&source.Kind{Type: &monitoringv1.Prometheus{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &marketplacev1alpha1.MeterBase{},
	})
	if err != nil {
		return err
	}

	// watch headless service
	err = c.Watch(&source.Kind{Type: &corev1.Service{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &marketplacev1alpha1.MeterBase{},
	})
	if err != nil {
		return err
	}

	return nil
}

// Reconcile reads that state of the cluster for a MeterBase object and makes changes based on the state read
// and what is in the MeterBase.Spec
func (r *ReconcileMeterBase) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling MeterBase")

	cc := r.ccprovider.NewCommandRunner(r.client, r.scheme, reqLogger)

	// Fetch the MeterBase instance
	instance := &marketplacev1alpha1.MeterBase{}
	result, _ := cc.Do(
		context.TODO(),
		GetAction(
			request.NamespacedName,
			instance,
		),
	)

	if result.Is(Error) || result.Is(NotFound) {
		if result.Is(NotFound) {
			reqLogger.Info("MeterBase resource not found. Ignoring since object must be deleted.")
		} else {
			reqLogger.Error(result.GetError(), "Failed to get MeterBase.")
		}
		return result.Return()
	}

	c := manifests.NewDefaultConfig()
	factory := manifests.NewFactory(instance.Namespace, c)

	// if instance.Enabled == false
	// return do nothing
	if !instance.Spec.Enabled {
		reqLogger.Info("MeterBase resource found but ignoring since metering is not enabled.")
		return reconcile.Result{}, nil
	}

	message := "Meter Base install starting"
	if instance.Status.Conditions == nil {
		instance.Status.Conditions = &status.Conditions{}
	}

	// find specific service monitor for kubelet
	openshiftKubeletMonitor := &monitoringv1.ServiceMonitor{}
	openshiftKubeStateMonitor := &monitoringv1.ServiceMonitor{}
	meteringKubeletMonitor := &monitoringv1.ServiceMonitor{}
	meteringKubeStateMonitor := &monitoringv1.ServiceMonitor{}

	prometheus := &monitoringv1.Prometheus{}
	prometheusService := &corev1.Service{}
	if result, err := cc.Do(context.TODO(),
		Do(r.reconcilePrometheusOperator(instance, factory)...),
		Do(r.reconcilePrometheus(instance, prometheus, factory)...),
		Do(r.reconcilePrometheusService(instance, prometheusService)...),
		Do(r.copyServiceMonitor(
			reqLogger,
			instance,
			types.NamespacedName{
				Namespace: "openshift-monitoring",
				Name:      "kubelet",
			},
			openshiftKubeletMonitor,
			types.NamespacedName{
				Namespace: instance.Namespace,
				Name:      "rhm-kubelet",
			},
			meteringKubeletMonitor,
		)...),
		Do(r.copyServiceMonitor(
			reqLogger,
			instance,
			types.NamespacedName{
				Namespace: "openshift-monitoring",
				Name:      "kube-state-metrics",
			},
			openshiftKubeStateMonitor,
			types.NamespacedName{
				Namespace: instance.Namespace,
				Name:      "rhm-kube-state-metrics",
			},
			meteringKubeStateMonitor,
		)...),
	); !result.Is(Continue) {

		if err != nil {
			reqLogger.Error(err, "error in reconcile")
			return result.ReturnWithError(merrors.Wrap(err, "error creating prometheus"))
		}

		return result.Return()
	}
	// ----
	// Update our status
	// ----

	if instance.Status.PrometheusStatus == nil ||
		!reflect.DeepEqual(instance.Status.PrometheusStatus, prometheus.Status) {
		reqLogger.Info("updating prometheus status")
		instance.Status.PrometheusStatus = prometheus.Status.DeepCopy()

		if result, err := cc.Do(context.TODO(), UpdateAction(instance, UpdateStatusOnly(true))); result.Is(Error) || result.Is(Requeue) {
			if err != nil {
				return result.ReturnWithError(merrors.Wrap(err, "error creating service monitor"))
			}

			return result.Return()
		}
	}

	message = "Meter Base install complete"
	if result, err := cc.Do(context.TODO(), UpdateStatusCondition(instance, instance.Status.Conditions, status.Condition{
		Type:    marketplacev1alpha1.ConditionInstalling,
		Status:  corev1.ConditionTrue,
		Reason:  marketplacev1alpha1.ReasonMeterBaseFinishInstall,
		Message: message,
	})); result.Is(Error) || result.Is(Requeue) {
		if err != nil {
			return result.ReturnWithError(merrors.Wrap(err, "error creating service monitor"))
		}

		return result.Return()
	}

	reqLogger.Info("finished reconciling")
	return reconcile.Result{}, nil
}

// configPath: /etc/config/prometheus.yml

type Images struct {
	ConfigmapReload string
	Server          string
}

type MeterbaseOpts struct {
	corev1.PullPolicy
	AssetPath string
}

func (r *ReconcileMeterBase) reconcilePrometheusSubscription(
	instance *marketplacev1alpha1.MeterBase,
	subscription *olmv1alpha1.Subscription,
) []ClientAction {
	newSub := &olmv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
		},
		Spec: &olmv1alpha1.SubscriptionSpec{
			Channel:                "beta",
			InstallPlanApproval:    olmv1alpha1.ApprovalAutomatic,
			Package:                "prometheus",
			CatalogSource:          "community-operators",
			CatalogSourceNamespace: "openshift-marketplace",
		},
	}

	return []ClientAction{
		HandleResult(
			GetAction(
				types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace},
				subscription,
			), OnNotFound(
				CreateAction(
					newSub,
					CreateWithAddOwner(instance),
				),
			)),
	}
}

func (r *ReconcileMeterBase) reconcilePrometheusOperator(
	instance *marketplacev1alpha1.MeterBase,
	factory *manifests.Factory,
) []ClientAction {
	cm := &corev1.ConfigMap{}
	deployment := &appsv1.Deployment{}
	service := &corev1.Service{}

	args := manifests.CreateOrUpdateFactoryItemArgs{
		Owner:          instance,
		PatchAnnotator: utils.RhmAnnotator,
		PatchChecker:   r.patchChecker,
	}

	return []ClientAction{
		manifests.CreateOrUpdateFactoryItemAction(
			cm,
			func() (runtime.Object, error) {
				return factory.NewPrometheusOperatorCertsCABundle()
			},
			args,
		),
		manifests.CreateOrUpdateFactoryItemAction(
			deployment,
			func() (runtime.Object, error) {
				return factory.NewPrometheusOperatorDeployment()
			},
			args,
		),
		manifests.CreateOrUpdateFactoryItemAction(
			service,
			func() (runtime.Object, error) {
				return factory.NewPrometheusOperatorService()
			},
			args,
		),
	}
}

func (r *ReconcileMeterBase) reconcilePrometheus(
	instance *marketplacev1alpha1.MeterBase,
	prometheus *monitoringv1.Prometheus,
	factory *manifests.Factory,
) []ClientAction {
	args := manifests.CreateOrUpdateFactoryItemArgs{
		Owner:          instance,
		PatchAnnotator: utils.RhmAnnotator,
		PatchChecker:   r.patchChecker,
	}

	kubeletCertsCM := &corev1.ConfigMap{}

	return []ClientAction{
		manifests.CreateOrUpdateFactoryItemAction(
			&corev1.ConfigMap{},
			func() (runtime.Object, error) {
				return factory.PrometheusServingCertsCABundle()
			},
			args,
		),
		HandleResult(
			GetAction(
				types.NamespacedName{Namespace: "openshift-config-managed", Name: "kubelet-serving-ca"},
				kubeletCertsCM,
			),
			OnNotFound(Call(func() (ClientAction, error) {
				return nil, merrors.New("require kubelet-serving-a configmap is not found")
			})),
			OnContinue(manifests.CreateOrUpdateFactoryItemAction(
				&corev1.ConfigMap{},
				func() (runtime.Object, error) {
					return factory.PrometheusKubeletServingCABundle(kubeletCertsCM.Data)
				},
				args,
			))),
		HandleResult(
			GetAction(
				types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace},
				prometheus,
			),
			OnNotFound(Call(r.createPrometheus(instance, factory))),
			OnContinue(Call(func() (ClientAction, error) {
				updatedPrometheus := prometheus.DeepCopy()
				expectedPrometheus, err := r.newPrometheusOperator(instance, r.opts, factory)

				if err != nil {
					return nil, merrors.Wrap(err, "error updating prometheus")
				}

				updatedPrometheus.Spec = expectedPrometheus.Spec

				update, err := r.patchChecker.CheckPatch(prometheus, updatedPrometheus)

				if err != nil {
					return nil, err
				}

				if !update {
					return nil, nil
				}

				updateResult := &ExecResult{}

				return HandleResult(
					StoreResult(
						updateResult,
						UpdateAction(updatedPrometheus, UpdateWithPatch(utils.RhmAnnotator))),
					OnError(
						Call(func() (ClientAction, error) {
							return UpdateStatusCondition(
								instance, instance.Status.Conditions, status.Condition{
									Type:    marketplacev1alpha1.ConditionError,
									Status:  corev1.ConditionFalse,
									Reason:  marketplacev1alpha1.ReasonMeterBasePrometheusInstall,
									Message: updateResult.Error(),
								}), nil
						})),
					OnRequeue(
						UpdateStatusCondition(instance, instance.Status.Conditions, status.Condition{
							Type:    marketplacev1alpha1.ConditionInstalling,
							Status:  corev1.ConditionTrue,
							Reason:  marketplacev1alpha1.ReasonMeterBasePrometheusInstall,
							Message: "updated prometheus",
						}))), nil
			}))),
	}
}

func (r *ReconcileMeterBase) reconcilePrometheusService(
	instance *marketplacev1alpha1.MeterBase,
	service *corev1.Service,
) []ClientAction {
	newService := r.serviceForPrometheus(instance, 9090)
	return []ClientAction{
		HandleResult(
			GetAction(
				types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace},
				service,
			),
			OnNotFound(CreateAction(newService, CreateWithAddOwner(instance)))),
	}
}

func (r *ReconcileMeterBase) createPrometheus(
	instance *marketplacev1alpha1.MeterBase,
	factory *manifests.Factory,
) func() (ClientAction, error) {
	return func() (ClientAction, error) {
		newProm, err := r.newPrometheusOperator(instance, r.opts, factory)
		createResult := &ExecResult{}

		if err != nil {
			return nil, merrors.Wrap(err, "error creating prometheus")
		}

		return HandleResult(
			StoreResult(
				createResult, CreateAction(
					newProm,
					CreateWithAddOwner(instance),
					CreateWithPatch(utils.RhmAnnotator),
				)),
			OnError(
				Call(func() (ClientAction, error) {
					return UpdateStatusCondition(instance, instance.Status.Conditions, status.Condition{
						Type:    marketplacev1alpha1.ConditionError,
						Status:  corev1.ConditionFalse,
						Reason:  marketplacev1alpha1.ReasonMeterBasePrometheusInstall,
						Message: createResult.Error(),
					}), nil
				})),
			OnRequeue(
				UpdateStatusCondition(instance, instance.Status.Conditions, status.Condition{
					Type:    marketplacev1alpha1.ConditionInstalling,
					Status:  corev1.ConditionTrue,
					Reason:  marketplacev1alpha1.ReasonMeterBasePrometheusInstall,
					Message: "created prometheus",
				})),
		), nil
	}
}

func (r *ReconcileMeterBase) newPrometheusOperator(
	cr *marketplacev1alpha1.MeterBase,
	opt *MeterbaseOpts,
	factory *manifests.Factory,
) (*monitoringv1.Prometheus, error) {
	ls := labelsForPrometheus(cr.Name)

	metadata := metav1.ObjectMeta{
		Name:      cr.Name,
		Namespace: cr.Namespace,
		Labels:    ls,
	}

	storageClass := ""
	if cr.Spec.Prometheus.Storage.Class == nil {
		foundDefaultClass, err := utils.GetDefaultStorageClass(r.client)

		if err != nil {
			return nil, err
		} else {
			storageClass = foundDefaultClass
		}
	} else {
		storageClass = *cr.Spec.Prometheus.Storage.Class
	}

	pvc, err := utils.NewPersistentVolumeClaim(utils.PersistentVolume{
		ObjectMeta: &metav1.ObjectMeta{
			Name: "storage-volume",
		},
		StorageClass: &storageClass,
		StorageSize:  &cr.Spec.Prometheus.Storage.Size,
	})

	if err != nil {
		return nil, err
	}

	prom, err := factory.NewPrometheusDeployment()
	prom.Name = cr.Name

	if err != nil {
		return nil, err
	}

	prom.ObjectMeta = metadata
	prom.Spec.Storage.VolumeClaimTemplate = pvc
	prom.Spec.Resources = cr.Spec.Prometheus.ResourceRequirements

	return prom, nil
}

func (r *ReconcileMeterBase) copyServiceMonitor(
	log logr.Logger,
	instance *marketplacev1alpha1.MeterBase,
	fromName types.NamespacedName,
	fromServiceMonitor *monitoringv1.ServiceMonitor,
	toName types.NamespacedName,
	toServiceMonitor *monitoringv1.ServiceMonitor,
) []ClientAction {
	return []ClientAction{
		HandleResult(
			GetAction(fromName, fromServiceMonitor),
			OnContinue(Call(func() (ClientAction, error) {
				newServiceMonitor := &monitoringv1.ServiceMonitor{
					ObjectMeta: metav1.ObjectMeta{
						Name:      toName.Name,
						Namespace: toName.Namespace,
						Labels:    labelsForServiceMonitor(fromServiceMonitor.Name, fromServiceMonitor.Namespace),
					},
					Spec: *fromServiceMonitor.Spec.DeepCopy(),
				}

				//newServiceMonitor.Spec.NamespaceSelector.MatchNames = []string{fromServiceMonitor.Namespace}
				log.Info("cloning from", "obj", fromServiceMonitor, "spec", fromServiceMonitor.Spec)
				log.Info("created obj", "obj", newServiceMonitor, "spec", newServiceMonitor.Spec)

				return HandleResult(
					GetAction(
						types.NamespacedName{Name: newServiceMonitor.Name, Namespace: newServiceMonitor.Namespace},
						toServiceMonitor,
					),
					OnNotFound(CreateAction(newServiceMonitor, CreateWithAddOwner(instance))),
					OnContinue(Call(func() (ClientAction, error) {
						update, err := r.patchChecker.CheckPatch(toServiceMonitor, newServiceMonitor)

						if err != nil {
							return nil, merrors.Wrap(err, "failed to calculate patch")
						}

						if !update {
							return nil, nil
						}

						return UpdateAction(newServiceMonitor, UpdateWithPatch(utils.RhmAnnotator)), nil
					}))), nil
			}))),
	}
}

// serviceForPrometheus function takes in a Prometheus object and returns a Service for that object.
func (r *ReconcileMeterBase) serviceForPrometheus(
	cr *marketplacev1alpha1.MeterBase,
	port int32) *corev1.Service {

	ser := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"prometheus": cr.Name,
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "web",
					Port:       port,
					TargetPort: intstr.FromString("web"),
				},
			},
			ClusterIP: "None",
		},
	}
	return ser
}

func (r *ReconcileMeterBase) newBaseConfigMap(filename string, cr *marketplacev1alpha1.MeterBase) (*corev1.ConfigMap, error) {
	int, err := utils.LoadYAML(filename, corev1.ConfigMap{})
	if err != nil {
		return nil, err
	}

	cfg := (int).(*corev1.ConfigMap)
	cfg.Namespace = cr.Namespace
	cfg.Name = cr.Name

	return cfg, nil
}

// labelsForPrometheus returns the labels for selecting the resources
// belonging to the given prometheus CR name.
func labelsForPrometheus(name string) map[string]string {
	return map[string]string{
		"app":          "meterbase-prom",
		"meterbase_cr": name,
		"prometheus":   "meterbase",
	}
}

// labelsForPrometheusOperator returns the labels for selecting the resources
// belonging to the given prometheus CR name.
func labelsForPrometheusOperator(name string) map[string]string {
	return map[string]string{"prometheus": name}
}

func labelsForServiceMonitor(name, namespace string) map[string]string {
	return map[string]string{
		"marketplace.redhat.com/metered":                  "true",
		"marketplace.redhat.com/deployed":                 "true",
		"marketplace.redhat.com/metered.kind":             "InternalServiceMonitor",
		"marketplace.redhat.com/serviceMonitor.Name":      name,
		"marketplace.redhat.com/serviceMonitor.Namespace": namespace,
	}
}
