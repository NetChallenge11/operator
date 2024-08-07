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

	// KServe
	servingv1alpha1 "github.com/kserve/kserve/pkg/apis/serving/v1alpha1"
	servingv1beta1 "github.com/kserve/kserve/pkg/apis/serving/v1beta1"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	devicesv1alpha1 "github.com/dcncompany/device-operator/api/v1alpha1"
)

// PartitionDecisionReconciler reconciles a PartitionDecision object
type PartitionDecisionReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=devices.devices.dcncompany.com,resources=partitiondecisions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=devices.devices.dcncompany.com,resources=partitiondecisions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=devices.devices.dcncompany.com,resources=partitiondecisions/finalizers,verbs=update

//+kubebuilder:rbac:groups=serving.kserve.io,resources=inferenceservices,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=serving.kserve.io,resources=inferenceservices/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=serving.kserve.io,resources=inferenceservices/finalizers,verbs=update

//+kubebuilder:rbac:groups=serving.kserve.io,resources=inferencegraphs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=serving.kserve.io,resources=inferencegraphs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=serving.kserve.io,resources=inferencegraphs/finalizers,verbs=update

func (r *PartitionDecisionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// PartitionDecision CR 가져오기
	partitionDecision := &devicesv1alpha1.PartitionDecision{}
	err := r.Get(ctx, req.NamespacedName, partitionDecision)
	if err != nil {
		if errors.IsNotFound(err) {
			// PartitionDecision 객체가 삭제됨, 관련 리소스를 삭제
			logger.Info(fmt.Sprintf("PartitionDecision '%s' not found, deleting associated resources", req.Name))

			// InferenceService 및 InferenceGraph 삭제
			deleteErr := r.deleteAssociatedResources(ctx, req.Namespace, req.Name)
			if deleteErr != nil {
				logger.Error(deleteErr, "Failed to delete associated resources")
				return ctrl.Result{}, deleteErr
			}

			return ctrl.Result{}, nil
		}
		// 다른 에러가 발생한 경우 에러 반환
		return ctrl.Result{}, fmt.Errorf("error fetching PartitionDecision: %w", err)
	}

	// AlgorithmName에 따른 작업 수행
	if partitionDecision.Spec.AlgorithmName == "ClientCPUBasedAlgorithm" {
		// CPU 사용량 기반으로 splitPoint 결정
		splitPoint, err := r.determineSplitPoint(partitionDecision.Spec.AlgorithmName)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to determine split point: %w", err)
		}

		services := []struct {
			name        string
			modelUri    string
			modelFormat string
		}{
			{
				name:        fmt.Sprintf("head-alexnet-split-layer-%d", splitPoint),
				modelUri:    "gs://kfserving-samples/models/tensorflow/flowers", //나중에 수정
				modelFormat: "tensorflow",
			},
			{
				name:        fmt.Sprintf("tail-alexnet-split-layer-%d", splitPoint),
				modelUri:    "gs://kfserving-samples/models/tensorflow/flowers", //나중에 수정
				modelFormat: "tensorflow",
			},
		}

		// Deploy each InferenceService
		for _, service := range services {
			if err := r.createAndApplyInferenceService(ctx, req.Namespace, service.name, service.modelUri, service.modelFormat); err != nil {
				if !errors.IsAlreadyExists(err) {
					return ctrl.Result{}, fmt.Errorf("failed to create or update InferenceService: %w", err)
				}
			}
		}

		// Define the graphs to be created
		graphs := []struct {
			name        string
			headService string
			tailService string
		}{
			{"head-tail-layer-1-chain", "head-alexnet-split-layer-1", "tail-alexnet-split-layer-1"},
			{"head-tail-layer-5-chain", "head-alexnet-split-layer-5", "tail-alexnet-split-layer-5"},
		}

		// Deploy each InferenceGraph
		for _, graph := range graphs {
			if err := r.createAndApplyInferenceGraph(ctx, req.Namespace, graph.name, graph.headService, graph.tailService); err != nil {
				if !errors.IsAlreadyExists(err) {
					return ctrl.Result{}, fmt.Errorf("failed to create or update InferenceGraph: %w", err)
				}
			}
		}

		// 상태 업데이트
		partitionDecision.Status.Ready = true
		partitionDecision.Status.LastUpdated = metav1.Now()
		partitionDecision.Status.CurrentPhase = "Deployed"
		if err := r.Status().Update(ctx, partitionDecision); err != nil {
			logger.Error(err, "Failed to update PartitionDecision status")
			return ctrl.Result{}, fmt.Errorf("failed to update PartitionDecision status: %w", err)
		}

	} else {
		// Unsupported algorithm
		return ctrl.Result{}, fmt.Errorf("unsupported algorithm name: %s", partitionDecision.Spec.AlgorithmName)
	}

	return ctrl.Result{}, nil
}

// Resource 삭제
func (r *PartitionDecisionReconciler) deleteAssociatedResources(ctx context.Context, namespace string, pdName string) error {
	// InferenceService 삭제
	inferenceServiceNames := []string{
		fmt.Sprintf("head-%s", pdName), // 실제 이름 확인 필요
		fmt.Sprintf("tail-%s", pdName), // 실제 이름 확인 필요
	}

	for _, isName := range inferenceServiceNames {
		is := &servingv1beta1.InferenceService{}
		isKey := types.NamespacedName{
			Name:      isName,
			Namespace: namespace,
		}

		err := r.Get(ctx, isKey, is)
		if err != nil {
			if errors.IsNotFound(err) {
				continue
			}
			return fmt.Errorf("failed to fetch InferenceService for deletion: %w", err)
		}

		err = r.Delete(ctx, is)
		if err != nil {
			return fmt.Errorf("failed to delete InferenceService: %w", err)
		}
	}

	// InferenceGraph 삭제
	inferenceGraphNames := []string{
		fmt.Sprintf("graph-%s", pdName), // 실제 이름 확인 필요
	}

	for _, igName := range inferenceGraphNames {
		ig := &servingv1alpha1.InferenceGraph{}
		igKey := types.NamespacedName{
			Name:      igName,
			Namespace: namespace,
		}

		err := r.Get(ctx, igKey, ig)
		if err != nil {
			if errors.IsNotFound(err) {
				continue
			}
			return fmt.Errorf("failed to fetch InferenceGraph for deletion: %w", err)
		}

		err = r.Delete(ctx, ig)
		if err != nil {
			return fmt.Errorf("failed to delete InferenceGraph: %w", err)
		}
	}

	return nil
}

// Algorithm에 따라 Split point 결정
// 현재는 CPU 사용량에 따른 분할 지점 결정 알고리즘 사용
func (r *PartitionDecisionReconciler) determineSplitPoint(algorithmName string) (int, error) {
	if algorithmName != "ClientCPUBasedAlgorithm" {
		return 0, fmt.Errorf("unsupported algorithm name: %s", algorithmName)
	}

	// 가상의 CPU 사용량 가져오기
	cpuUsage := getServerCPUUsage()

	// CPU 사용량에 따른 splitPoint 결정
	var splitPoint int
	if cpuUsage < 70 {
		splitPoint = 5
	} else {
		splitPoint = 1
	}

	return splitPoint, nil
}

func getServerCPUUsage() int {
	// 예시로 고정된 값 반환 (나중에 업데이트 해야 함)
	return 65
}

func (r *PartitionDecisionReconciler) createAndApplyInferenceService(ctx context.Context, namespace, serviceName, modelUri string, modelFormat string) error {
	logger := log.FromContext(ctx)

	// Check if the model format is tensorflow
	if modelFormat != "tensorflow" {
		err := fmt.Errorf("unsupported model format: %s, only 'tensorflow' is supported", modelFormat)
		logger.Error(err, "Failed to create InferenceService due to unsupported model format", "name", serviceName)
		return err
	}

	// InferenceService 객체 정의
	inferenceService := &servingv1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: namespace,
		},
		Spec: servingv1beta1.InferenceServiceSpec{
			Predictor: servingv1beta1.PredictorSpec{
				Tensorflow: &servingv1beta1.TFServingSpec{
					PredictorExtensionSpec: servingv1beta1.PredictorExtensionSpec{
						StorageURI: &modelUri,
					},
				},
			},
		},
	}

	// InferenceService 생성 또는 업데이트 시도
	err := r.Client.Create(ctx, inferenceService)
	if err != nil {
		if errors.IsAlreadyExists(err) {
			// 이미 존재하는 경우, 기존 InferenceService를 업데이트
			existingService := &servingv1beta1.InferenceService{}
			err = r.Client.Get(ctx, types.NamespacedName{Name: serviceName, Namespace: namespace}, existingService)
			if err != nil {
				logger.Error(err, "기존 InferenceService를 가져오는 데 실패", "name", serviceName)
				return fmt.Errorf("기존 InferenceService를 가져오는 데 실패: %w", err)
			}

			existingService.Spec = inferenceService.Spec
			err = r.Client.Update(ctx, existingService)
			if err != nil {
				logger.Error(err, "기존 InferenceService를 업데이트하는 데 실패", "name", serviceName)
				return fmt.Errorf("기존 InferenceService를 업데이트하는 데 실패: %w", err)
			}

			logger.Info("기존 InferenceService를 업데이트하였습니다", "name", serviceName)
		} else {
			return fmt.Errorf("InferenceService를 생성하는 데 실패: %w", err)
		}
	} else {
		logger.Info("새로운 InferenceService를 생성하였습니다", "name", serviceName)
	}

	return nil
}

func (r *PartitionDecisionReconciler) createAndApplyInferenceGraph(ctx context.Context, namespace, graphName, headService, tailService string) error {
	logger := log.FromContext(ctx)

	// InferenceGraph 객체 정의
	inferenceGraph := &servingv1alpha1.InferenceGraph{
		ObjectMeta: metav1.ObjectMeta{
			Name:      graphName,
			Namespace: namespace,
		},
		Spec: servingv1alpha1.InferenceGraphSpec{
			Nodes: map[string]servingv1alpha1.InferenceRouter{
				"root": {
					RouterType: servingv1alpha1.Sequence,
					Steps: []servingv1alpha1.InferenceStep{
						{
							InferenceTarget: servingv1alpha1.InferenceTarget{
								ServiceName: headService,
							},
							Data: "$request",
						},
						{
							InferenceTarget: servingv1alpha1.InferenceTarget{
								ServiceName: tailService,
							},
							Data: "$response",
						},
					},
				},
			},
			MinReplicas:    ptrInt(1),
			MaxReplicas:    3,
			TimeoutSeconds: ptrInt64(60),
		},
	}

	// InferenceGraph 생성 또는 업데이트 시도
	err := r.Client.Create(ctx, inferenceGraph)
	if err != nil {
		if errors.IsAlreadyExists(err) {
			// 이미 존재하는 경우, 기존 InferenceGraph를 업데이트
			existingGraph := &servingv1alpha1.InferenceGraph{}
			err = r.Client.Get(ctx, types.NamespacedName{Name: graphName, Namespace: namespace}, existingGraph)
			if err != nil {
				return fmt.Errorf("기존 InferenceGraph를 가져오는 데 실패: %w", err)
			}

			existingGraph.Spec = inferenceGraph.Spec
			err = r.Client.Update(ctx, existingGraph)
			if err != nil {
				return fmt.Errorf("기존 InferenceGraph를 업데이트하는 데 실패: %w", err)
			}

			logger.Info("기존 InferenceGraph를 업데이트하였습니다", "name", graphName)
		} else {
			return fmt.Errorf("InferenceGraph를 생성하는 데 실패: %w", err)
		}
	} else {
		logger.Info("새로운 InferenceGraph를 생성하였습니다", "name", graphName)
	}

	return nil
}

// 헬퍼 함수: 포인터 변환
func ptrInt(i int) *int       { return &i }
func ptrInt64(i int64) *int64 { return &i }

// SetupWithManager sets up the controller with the Manager.
func (r *PartitionDecisionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&devicesv1alpha1.PartitionDecision{}).
		Complete(r)
}
