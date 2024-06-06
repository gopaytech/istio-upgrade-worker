package statefulset

import (
	"context"
	"log"
	"time"

	"github.com/gopaytech/istio-upgrade-worker/config"
	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const restartedAtAnnotations = "kubectl.kubernetes.io/restartedAt"

type svc struct {
	k8sConfig *config.Conf
}

type Service interface {
	RolloutRestart(ctx context.Context, namespace string, stsName string) error
	FindByNamespace(ctx context.Context, namespace string) ([]v1.StatefulSet, error)
}

func New(cfg *config.Conf) Service {
	return &svc{
		k8sConfig: cfg,
	}
}

func (s *svc) FindByNamespace(ctx context.Context, namespace string) ([]v1.StatefulSet, error) {
	c := s.k8sConfig.K8SClient()
	stsList, err := c.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Println("failed getting statefulsets: ", err.Error())
		return nil, err
	}
	return stsList.Items, nil
}

func (s *svc) RolloutRestart(ctx context.Context, namespace string, stsName string) error {
	c := s.k8sConfig.K8SClient()
	sts, err := c.AppsV1().StatefulSets(namespace).Get(ctx, stsName, metav1.GetOptions{})
	if err != nil {
		log.Println("failed getting statefulset by name: ", err.Error())
		return err
	}
	if sts.Spec.Template.ObjectMeta.Annotations == nil {
		sts.Spec.Template.ObjectMeta.Annotations = make(map[string]string)
	}
	sts.Spec.Template.ObjectMeta.Annotations[restartedAtAnnotations] = time.Now().Format(time.RFC3339)
	_, err = c.AppsV1().StatefulSets(namespace).Update(ctx, sts, metav1.UpdateOptions{})
	return err
}
