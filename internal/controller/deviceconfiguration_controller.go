/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	devicesv1alpha1 "github.com/dcncompany/device-operator/api/v1alpha1"
)

// DeviceConfigurationReconciler reconciles a DeviceConfiguration object
type DeviceConfigurationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=devices.devices.dcncompany.com,resources=deviceconfigurations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=devices.devices.dcncompany.com,resources=deviceconfigurations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=devices.devices.dcncompany.com,resources=deviceconfigurations/finalizers,verbs=update

//+kubebuilder:rbac:groups=devices.devices.dcncompany.com,resources=partitiondecisions,verbs=get;list;watch;create;update;patch;delete

func (r *DeviceConfigurationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// DeviceConfiguration 인스턴스 가져오기
	deviceConfig := &devicesv1alpha1.DeviceConfiguration{}
	err := r.Get(ctx, req.NamespacedName, deviceConfig)
	if err != nil {
		if errors.IsNotFound(err) {
			// 객체를 찾을 수 없는 경우 추가 처리 없음
			return ctrl.Result{}, nil
		}
		// 객체 읽기 오류
		return ctrl.Result{}, fmt.Errorf("DeviceConfiguration을 가져오는 중 오류 발생: %w", err)
	}

	// DeviceConfiguration 인스턴스가 삭제될 예정인지 확인
	if deviceConfig.ObjectMeta.DeletionTimestamp.IsZero() {
		// Finalizer가 없으면 추가
		if !containsString(deviceConfig.GetFinalizers(), deviceConfigurationFinalizer) {
			controllerutil.AddFinalizer(deviceConfig, deviceConfigurationFinalizer)
			if err := r.Update(ctx, deviceConfig); err != nil {
				return ctrl.Result{}, fmt.Errorf("finalizer 추가 실패: %w", err)
			}
		}
	} else {
		// DeviceConfiguration이 삭제 중일 때 처리
		if containsString(deviceConfig.GetFinalizers(), deviceConfigurationFinalizer) {
			result, err := r.deleteAssociatedPartitionDecisions(ctx, req.Namespace, deviceConfig.Spec.ClientClusters)
			if err != nil {
				logger.Error(err, "Failed to delete associated PartitionDecisions")
				return result, fmt.Errorf("failed to delete associated PartitionDecisions: %w", err)
			}

			controllerutil.RemoveFinalizer(deviceConfig, deviceConfigurationFinalizer)
			if err := r.Update(ctx, deviceConfig); err != nil {
				return ctrl.Result{}, fmt.Errorf("finalizer 제거 실패: %w", err)
			}
		}

		// 객체가 삭제 중이므로 조정 중지
		return ctrl.Result{}, nil
	}

	// 기존 조정 로직
	for _, clusterName := range deviceConfig.Spec.ClientClusters {
		err := r.ensurePartitionDecision(ctx, req.Namespace, deviceConfig, clusterName)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	// 상태 업데이트
	deviceConfig.Status.LastUpdated = metav1.Now()
	deviceConfig.Status.Ready = true
	deviceConfig.Status.CurrentPhase = "Reconciled"
	if err := r.Status().Update(ctx, deviceConfig); err != nil {
		logger.Error(err, "Failed to update DeviceConfiguration status")
		return ctrl.Result{}, fmt.Errorf("DeviceConfiguration 상태 업데이트 실패: %w", err)
	}

	return ctrl.Result{}, nil
}

// Helper 함수: 슬라이스에 문자열이 있는지 확인
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// PartitionDecision 생성 및 업데이트를 위한 함수
func (r *DeviceConfigurationReconciler) ensurePartitionDecision(ctx context.Context, namespace string, deviceConfig *devicesv1alpha1.DeviceConfiguration, clusterName string) error {
	pd := &devicesv1alpha1.PartitionDecision{}
	pdName := fmt.Sprintf("pd-%s", clusterName)

	err := r.Get(ctx, client.ObjectKey{Namespace: namespace, Name: pdName}, pd)
	if err != nil {
		if errors.IsNotFound(err) {
			// PartitionDecision CR 생성
			newPD := r.createPartitionDecision(ctx, namespace, deviceConfig, clusterName) // Corrected function call
			if err := r.Create(ctx, newPD); err != nil {
				return fmt.Errorf("error creating PartitionDecision: %w", err)
			}
			return nil
		}
		return fmt.Errorf("error fetching PartitionDecision: %w", err)
	}

	// PartitionDecision 업데이트 (필요한 경우)
	updated := false
	if pd.Spec.AlgorithmName != deviceConfig.Spec.AlgorithmName {
		pd.Spec.AlgorithmName = deviceConfig.Spec.AlgorithmName
		updated = true
	}
	if pd.Spec.ModelName != deviceConfig.Spec.ModelName {
		pd.Spec.ModelName = deviceConfig.Spec.ModelName
		updated = true
	}

	// 다른 스펙 필드도 필요에 따라 업데이트
	if updated {
		if err := r.Update(ctx, pd); err != nil {
			return fmt.Errorf("error updating PartitionDecision: %w", err)
		}
	}

	return nil
}

// PartitionDecision 객체 생성 로직
func (r *DeviceConfigurationReconciler) createPartitionDecision(_ context.Context, namespace string, deviceConfig *devicesv1alpha1.DeviceConfiguration, clusterName string) *devicesv1alpha1.PartitionDecision {
	return &devicesv1alpha1.PartitionDecision{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("pd-%s", clusterName),
			Namespace: namespace,
		},
		Spec: devicesv1alpha1.PartitionDecisionSpec{
			AlgorithmName:     deviceConfig.Spec.AlgorithmName,
			ModelName:         deviceConfig.Spec.ModelName,
			ClientClusterName: []string{clusterName},
			MetricTypes:       "defaultMetric",  // 예시 메트릭 종류
			ChannelName:       "defaultChannel", // 예시 채널 이름
		},
	}
}

// DeviceConfiguration을 위한 Finalizer 문자열 추가
const deviceConfigurationFinalizer = "devices.devices.dcncompany.com/finalizer"

// Delete PartitionDecision CRs
func (r *DeviceConfigurationReconciler) deleteAssociatedPartitionDecisions(ctx context.Context, namespace string, clientClusters []string) (ctrl.Result, error) {
	for _, clusterName := range clientClusters {
		pd := &devicesv1alpha1.PartitionDecision{}
		pdKey := types.NamespacedName{
			Name:      fmt.Sprintf("pd-%s", clusterName),
			Namespace: namespace,
		}

		err := r.Get(ctx, pdKey, pd)
		if err != nil {
			if errors.IsNotFound(err) {
				// 이미 삭제된 경우
				continue
			}
			return ctrl.Result{}, fmt.Errorf("error fetching PartitionDecision for deletion: %w", err)
		}

		// Delete the found PartitionDecision
		err = r.Delete(ctx, pd)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("error deleting PartitionDecision: %w", err)
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DeviceConfigurationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&devicesv1alpha1.DeviceConfiguration{}).
		Complete(r)
}
