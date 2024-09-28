package kubernetes

import (
	"context"
	"log"
	"time"

	"github.com/gopaytech/istio-upgrade-worker/config"
	v1 "k8s.io/api/apps/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	deploymentRestartedAtAnnotations = "kubectl.kubernetes.io/restartedAt"
)

type DeploymentInterface interface {
	RolloutRestart(ctx context.Context, namespace string, deplName string) error
	FindByNamespace(ctx context.Context, namespace string) ([]v1.Deployment, error)
}

func NewDeploymentService(kubernetesConfig *config.Kubernetes) DeploymentInterface {
	return &DeploymentService{
		kubernetesConfig: kubernetesConfig,
	}
}

type DeploymentService struct {
	kubernetesConfig *config.Kubernetes
}

func (s *DeploymentService) RolloutRestart(ctx context.Context, namespace string, deplName string) error {
	c := s.kubernetesConfig.Client()
	depl, err := c.AppsV1().Deployments(namespace).Get(ctx, deplName, metav1.GetOptions{})
	if err != nil {
		log.Println(err)
		return err
	}
	if depl.Spec.Template.ObjectMeta.Annotations == nil {
		depl.Spec.Template.ObjectMeta.Annotations = make(map[string]string)
	}
	depl.Spec.Template.ObjectMeta.Annotations[deploymentRestartedAtAnnotations] = time.Now().Format(time.RFC3339)
	_, err = c.AppsV1().Deployments(namespace).Update(ctx, depl, metav1.UpdateOptions{})
	return err
}

func (s *DeploymentService) FindByNamespace(ctx context.Context, namespace string) ([]v1.Deployment, error) {
	c := s.kubernetesConfig.Client()
	deploymentList, err := c.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	deployments := deploymentList.Items
	return deployments, nil
}
