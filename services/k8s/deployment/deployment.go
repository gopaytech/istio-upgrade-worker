package deployment

import (
	"context"
	"log"
	"time"

	v1 "k8s.io/api/apps/v1"

	"github.com/gopaytech/istio-upgrade-worker/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	restartedAtAnnotations = "kubectl.kubernetes.io/restartedAt"
)

type Service interface {
	RolloutRestart(ctx context.Context, namespace string, deplName string) error
	FindByNamespace(ctx context.Context, namespace string) ([]v1.Deployment, error)
}

type svc struct {
	k8sConfig *config.Conf
}

func New(cfg *config.Conf) Service {
	return &svc{
		k8sConfig: cfg,
	}
}

func (s *svc) RolloutRestart(ctx context.Context, namespace string, deplName string) error {
	c := s.k8sConfig.K8SClient()
	depl, err := c.AppsV1().Deployments(namespace).Get(ctx, deplName, metav1.GetOptions{})
	if err != nil {
		log.Println(err)
		return err
	}
	if depl.Spec.Template.ObjectMeta.Annotations == nil {
		depl.Spec.Template.ObjectMeta.Annotations = make(map[string]string)
	}
	depl.Spec.Template.ObjectMeta.Annotations[restartedAtAnnotations] = time.Now().Format(time.RFC3339)
	_, err = c.AppsV1().Deployments(namespace).Update(ctx, depl, metav1.UpdateOptions{})
	return err
}

func (s *svc) FindByNamespace(ctx context.Context, namespace string) ([]v1.Deployment, error) {
	c := s.k8sConfig.K8SClient()
	deploymentList, err := c.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	deployments := deploymentList.Items
	return deployments, nil
}
