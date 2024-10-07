package deployment

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/rancher/rancher/tests/v2/actions/kubeapi/workloads/deployments"
	"github.com/rancher/rancher/tests/v2/actions/workloads/pods"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/kubectl"
	"github.com/rancher/shepherd/extensions/workloads"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	active              = "active"
	defaultNamespace    = "default"
	port                = "port"
	DeploymentSteveType = "apps.deployment"
	imageName           = "nginx"
	historyHeaderLength = 2
	revisionNumberIndex = 0
	historyHeader       = "REVISION  CHANGE-CAUSE"
	revisionsIndex      = 1
	revisionAnnotation  = "deployment.kubernetes.io/revision"
)

// CreateDeployment is a helper to create a deployment with or without a secret/configmap
func CreateDeployment(client *rancher.Client, clusterID, namespaceName string, replicaCount int, secretName, configMapName string, useEnvVars, useVolumes bool) (*appv1.Deployment, error) {
	deploymentName := namegen.AppendRandomString("testdeployment")
	containerName := namegen.AppendRandomString("testcontainer")
	pullPolicy := corev1.PullAlways
	replicas := int32(replicaCount)

	var podTemplate corev1.PodTemplateSpec

	if secretName != "" || configMapName != "" {
		podTemplate = pods.NewPodTemplateWithConfig(secretName, configMapName, useEnvVars, useVolumes)
	} else {
		containerTemplate := workloads.NewContainer(
			containerName,
			imageName,
			pullPolicy,
			[]corev1.VolumeMount{},
			[]corev1.EnvFromSource{},
			nil,
			nil,
			nil,
		)
		podTemplate = workloads.NewPodTemplate(
			[]corev1.Container{containerTemplate},
			[]corev1.Volume{},
			[]corev1.LocalObjectReference{},
			nil,
			nil,
		)
	}

	createdDeployment, err := deployments.CreateDeployment(client, clusterID, deploymentName, namespaceName, podTemplate, replicas)
	if err != nil {
		return nil, err
	}

	return createdDeployment, err
}

// UpdateDeploymentContainer is a helper to update containers in the deployment
func UpdateDeploymentContainer(client *rancher.Client, clusterID, namespaceName string, deploymentName string, containerTemplate corev1.Container) (*appv1.Deployment, error) {
	wranglerContext, err := client.WranglerContext.DownStreamClusterWranglerContext(clusterID)
	if err != nil {
		return nil, err
	}

	latestDeployment, err := wranglerContext.Apps.Deployment().Get(namespaceName, deploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	latestDeployment.Spec.Template.Spec.Containers = append(latestDeployment.Spec.Template.Spec.Containers, containerTemplate)

	updatedDeployment, err := wranglerContext.Apps.Deployment().Update(latestDeployment)
	if err != nil {
		return nil, err
	}

	return updatedDeployment, err
}

// UpdateDeployment is a helper to update deployments
func UpdateDeployment(client *rancher.Client, clusterID, namespaceName string, deployment *appv1.Deployment) (*appv1.Deployment, error) {
	wranglerContext, err := client.WranglerContext.DownStreamClusterWranglerContext(clusterID)
	if err != nil {
		return nil, err
	}

	latestDeployment, err := wranglerContext.Apps.Deployment().Get(namespaceName, deployment.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	deployment.ResourceVersion = latestDeployment.ResourceVersion

	updatedDeployment, err := wranglerContext.Apps.Deployment().Update(deployment)
	if err != nil {
		return nil, err
	}

	return updatedDeployment, err
}

// RolbackDeployment is a helper to rollback deployments
func RollbackDeployment(client *rancher.Client, clusterID, namespaceName string, deploymentName string, revision int) (string, error) {
	deploymentCmd := fmt.Sprintf("deployment.apps/%s", deploymentName)
	revisionCmd := fmt.Sprintf("--to-revision=%s", strconv.Itoa(revision))
	execCmd := []string{"kubectl", "rollout", "undo", "-n", namespaceName, deploymentCmd, revisionCmd}
	logCmd, err := kubectl.Command(client, nil, clusterID, execCmd, "")
	return logCmd, err
}

// ValidateRolloutHistoryDeployment is a helper to validate rollout history deployment
func ValidateRolloutHistoryDeployment(client *rancher.Client, clusterID, namespaceName string, deploymentName string, expectedRevision string) error {
	wranglerContext, err := client.WranglerContext.DownStreamClusterWranglerContext(clusterID)
	if err != nil {
		return err
	}

	latestDeployment, err := wranglerContext.Apps.Deployment().Get(namespaceName, deploymentName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if latestDeployment.ObjectMeta.Annotations == nil {
		return errors.New("revision empty")
	}

	revision := latestDeployment.ObjectMeta.Annotations[revisionAnnotation]

	if revision == expectedRevision {
		return nil
	}

	return errors.New("revision not found")
}

// DeleteDeployment is a helper to delete a deployment
func DeleteDeployment(client *rancher.Client, clusterID string, deployment *appv1.Deployment) error {
	steveClient, err := client.Steve.ProxyDownstream(clusterID)
	if err != nil {
		return err
	}

	deploymentID := deployment.Namespace + "/" + deployment.Name
	deploymentResp, err := steveClient.SteveType(DeploymentSteveType).ByID(deploymentID)
	if err != nil {
		return err
	}

	err = steveClient.SteveType(DeploymentSteveType).Delete(deploymentResp)
	if err != nil {
		return err
	}

	return nil
}
