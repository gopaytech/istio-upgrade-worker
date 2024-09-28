package kubernetes

import (
	"context"
	"log"
	"time"

	"github.com/gopaytech/istio-upgrade-worker/config"
	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const statefulSetRestartedAtAnnotations = "kubectl.kubernetes.io/restartedAt"

type StatefulSetInterface interface {
	RolloutRestart(ctx context.Context, namespace string, stsName string) error
	FindByNamespace(ctx context.Context, namespace string) ([]v1.StatefulSet, error)
}

func NewStatefulSetService(kubernetesConfig *config.Kubernetes) StatefulSetInterface {
	return &StatefulSetService{
		kubernetesConfig: kubernetesConfig,
	}
}

type StatefulSetService struct {
	kubernetesConfig *config.Kubernetes
}

func (s *StatefulSetService) FindByNamespace(ctx context.Context, namespace string) ([]v1.StatefulSet, error) {
	c := s.kubernetesConfig.Client()
	stsList, err := c.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Println("failed getting statefulsets: ", err.Error())
		return nil, err
	}
	return stsList.Items, nil
}

func (s *StatefulSetService) RolloutRestart(ctx context.Context, namespace string, stsName string) error {
	c := s.kubernetesConfig.Client()
	sts, err := c.AppsV1().StatefulSets(namespace).Get(ctx, stsName, metav1.GetOptions{})
	if err != nil {
		log.Println("failed getting statefulset by name: ", err.Error())
		return err
	}
	if sts.Spec.Template.ObjectMeta.Annotations == nil {
		sts.Spec.Template.ObjectMeta.Annotations = make(map[string]string)
	}
	sts.Spec.Template.ObjectMeta.Annotations[statefulSetRestartedAtAnnotations] = time.Now().Format(time.RFC3339)
	_, err = c.AppsV1().StatefulSets(namespace).Update(ctx, sts, metav1.UpdateOptions{})
	return err
}
