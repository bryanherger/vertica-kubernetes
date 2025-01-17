/*
 (c) Copyright [2021] Micro Focus or one of its affiliates.
 Licensed under the Apache License, Version 2.0 (the "License");
 You may not use this file except in compliance with the License.
 You may obtain a copy of the License at

 http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package controllers

import (
	"fmt"

	vapi "github.com/vertica/vertica-kubernetes/api/v1beta1"
	"github.com/vertica/vertica-kubernetes/pkg/names"
	"github.com/vertica/vertica-kubernetes/pkg/paths"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	LicensingMountName = "licensing"
	PodInfoMountName   = "podinfo"
)

// buildExtSvc creates desired spec for the external service.
func buildExtSvc(nm types.NamespacedName, vdb *vapi.VerticaDB, sc *vapi.Subcluster) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        nm.Name,
			Namespace:   nm.Namespace,
			Labels:      makeLabelsForSvcObject(vdb, sc, "external"),
			Annotations: makeAnnotationsForObject(vdb),
		},
		Spec: corev1.ServiceSpec{
			Selector: makeSvcSelectorLabels(vdb, sc),
			Type:     sc.ServiceType,
			Ports: []corev1.ServicePort{
				{Port: 5433, Name: "vertica", NodePort: sc.NodePort},
				{Port: 5444, Name: "agent"},
			},
			ExternalIPs: sc.ExternalIPs,
		},
	}
}

// buildHlSvc creates the desired spec for the headless service.
func buildHlSvc(nm types.NamespacedName, vdb *vapi.VerticaDB) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        nm.Name,
			Namespace:   nm.Namespace,
			Labels:      makeLabelsForSvcObject(vdb, nil, "headless"),
			Annotations: makeAnnotationsForObject(vdb),
		},
		Spec: corev1.ServiceSpec{
			Selector:                 makeSvcSelectorLabels(vdb, nil),
			ClusterIP:                "None",
			Type:                     "ClusterIP",
			PublishNotReadyAddresses: true,
			Ports: []corev1.ServicePort{
				{Port: 22, Name: "ssh"},
			},
		},
	}
}

// buildVolumeMounts returns the volume mounts to include in the sts pod spec
func buildVolumeMounts(vdb *vapi.VerticaDB) []corev1.VolumeMount {
	volMnts := []corev1.VolumeMount{
		{Name: LocalDataPVC, MountPath: paths.LocalDataPath},
		{Name: LocalDataPVC, SubPath: paths.GetPVSubPath(vdb, "config"), MountPath: paths.ConfigPath},
		{Name: LocalDataPVC, SubPath: paths.GetPVSubPath(vdb, "log"), MountPath: paths.LogPath},
		{Name: LocalDataPVC, SubPath: paths.GetPVSubPath(vdb, "data"), MountPath: vdb.Spec.Local.DataPath},
		{Name: LocalDataPVC, SubPath: paths.GetPVSubPath(vdb, "depot"), MountPath: vdb.Spec.Local.DepotPath},
		{Name: PodInfoMountName, MountPath: paths.PodInfoPath},
	}

	if vdb.Spec.LicenseSecret != "" {
		volMnts = append(volMnts, corev1.VolumeMount{
			Name:      LicensingMountName,
			MountPath: paths.MountedLicensePath,
		})
	}

	return volMnts
}

// buildVolumes builds up a list of volumes to include in the sts
func buildVolumes(vdb *vapi.VerticaDB) []corev1.Volume {
	vols := []corev1.Volume{}
	vols = append(vols, buildPodInfoVolume())
	if vdb.Spec.LicenseSecret != "" {
		vols = append(vols, buildLicenseVolume(vdb))
	}
	return vols
}

// buildLicenseVolume returns a volume that contains any licenses
func buildLicenseVolume(vdb *vapi.VerticaDB) corev1.Volume {
	return corev1.Volume{
		Name: LicensingMountName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: vdb.Spec.LicenseSecret,
			},
		},
	}
}

// buildPodInfoVolume constructs the volume that has the /etc/podinfo files.
func buildPodInfoVolume() corev1.Volume {
	return corev1.Volume{
		Name: PodInfoMountName,
		VolumeSource: corev1.VolumeSource{
			Projected: &corev1.ProjectedVolumeSource{
				Sources: []corev1.VolumeProjection{
					{
						DownwardAPI: &corev1.DownwardAPIProjection{
							Items: []corev1.DownwardAPIVolumeFile{
								{
									Path: "memory-limit",
									ResourceFieldRef: &corev1.ResourceFieldSelector{
										Resource:      "limits.memory",
										ContainerName: ServerContainer,
									},
								},
								{
									Path: "memory-request",
									ResourceFieldRef: &corev1.ResourceFieldSelector{
										Resource:      "requests.memory",
										ContainerName: ServerContainer,
									},
								},
								{
									Path: "cpu-limit",
									ResourceFieldRef: &corev1.ResourceFieldSelector{
										Resource:      "limits.cpu",
										ContainerName: ServerContainer,
									},
								},
								{
									Path: "cpu-request",
									ResourceFieldRef: &corev1.ResourceFieldSelector{
										Resource:      "requests.cpu",
										ContainerName: ServerContainer,
									},
								},
								{
									Path: "labels",
									FieldRef: &corev1.ObjectFieldSelector{
										FieldPath: "metadata.labels",
									},
								},
								{
									Path: "name",
									FieldRef: &corev1.ObjectFieldSelector{
										FieldPath: "metadata.name",
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// buildPodSpec creates a PodSpec for the statefulset
func buildPodSpec(vdb *vapi.VerticaDB, sc *vapi.Subcluster) corev1.PodSpec {
	termGracePeriod := int64(0)
	return corev1.PodSpec{
		NodeSelector: sc.NodeSelector,
		Affinity:     sc.Affinity,
		Tolerations:  sc.Tolerations,
		Containers: []corev1.Container{
			{
				Image:           vdb.Spec.Image,
				ImagePullPolicy: vdb.Spec.ImagePullPolicy,
				Name:            ServerContainer,
				Resources:       sc.Resources,
				Ports: []corev1.ContainerPort{
					{ContainerPort: 5433, Name: "vertica"},
					{ContainerPort: 5434, Name: "vertica-int"},
					{ContainerPort: 22, Name: "ssh"},
				},
				ReadinessProbe: &corev1.Probe{
					Handler: corev1.Handler{
						Exec: &corev1.ExecAction{
							Command: []string{"bash", "-c",
								fmt.Sprintf("vertica --status -D %s/%s/v_*_catalog",
									vdb.Spec.Local.DataPath, vdb.Spec.DBName)},
						},
					},
				},
				Env: []corev1.EnvVar{
					{Name: "POD_IP", ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{FieldPath: "status.podIP"},
					}},
				},
				VolumeMounts: buildVolumeMounts(vdb),
			},
		},
		Volumes:                       buildVolumes(vdb),
		TerminationGracePeriodSeconds: &termGracePeriod,
	}
}

// getStorageClassName returns a  pointer to the StorageClass
func getStorageClassName(vdb *vapi.VerticaDB) *string {
	if vdb.Spec.Local.StorageClass == "" {
		return nil
	}
	return &vdb.Spec.Local.StorageClass
}

// buildStsSpec builds manifest for a subclusters statefulset
func buildStsSpec(nm types.NamespacedName, vdb *vapi.VerticaDB, sc *vapi.Subcluster) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        nm.Name,
			Namespace:   nm.Namespace,
			Labels:      makeLabelsForObject(vdb, sc),
			Annotations: makeAnnotationsForObject(vdb),
		},
		Spec: appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: makeSvcSelectorLabels(vdb, sc),
			},
			ServiceName: names.GenHlSvcName(vdb).Name,
			Replicas:    &sc.Size,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      makeLabelsForObject(vdb, sc),
					Annotations: makeAnnotationsForObject(vdb),
				},
				Spec: buildPodSpec(vdb, sc),
			},
			PodManagementPolicy: appsv1.ParallelPodManagement,
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: LocalDataPVC,
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						StorageClassName: getStorageClassName(vdb),
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								"storage": vdb.Spec.Local.RequestSize,
							},
						},
					},
				},
			},
		},
	}
}

// buildPod will construct a spec for a pod.
// This is only here for testing purposes when we need to construct the pods ourselves.  This
// bit is typically handled by the statefulset controller.
func buildPod(vdb *vapi.VerticaDB, sc *vapi.Subcluster, podIndex int32) *corev1.Pod {
	nm := names.GenPodName(vdb, sc, podIndex)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nm.Name,
			Namespace: nm.Namespace,
		},
		Spec: buildPodSpec(vdb, sc),
	}
	// Set a few things in the spec that are normally done by the statefulset
	// controller. Again, this is for testing purposes only as the statefulset
	// controller handles adding of the PVC to the volume list.
	pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
		Name: LocalDataPVC,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: LocalDataPVC + "-" + vdb.ObjectMeta.Name + "-" + sc.Name + fmt.Sprintf("%d", podIndex),
			},
		},
	})
	pod.Spec.Hostname = nm.Name
	pod.Spec.Subdomain = names.GenHlSvcName(vdb).Name
	return pod
}

// buildCommunalCredSecret is a test helper to build up the Secret spec to store communal credentials
func buildCommunalCredSecret(vdb *vapi.VerticaDB, accessKey, secretKey string) *corev1.Secret {
	nm := names.GenCommunalCredSecretName(vdb)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nm.Name,
			Namespace: nm.Namespace,
		},
		Data: map[string][]byte{
			S3AccessKeyName: []byte(accessKey),
			S3SecretKeyName: []byte(secretKey),
		},
	}
	return secret
}
