package api

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// podSpecForYAML is a K8s-like structure for serializing Pod to YAML (Pod Specification view).
type podSpecForYAML struct {
	APIVersion string                 `yaml:"apiVersion"`
	Kind       string                 `yaml:"kind"`
	Metadata   map[string]interface{} `yaml:"metadata"`
	Spec       map[string]interface{} `yaml:"spec"`
}

func buildPodSpecYAML(pod *models.Pod) ([]byte, error) {
	meta := map[string]interface{}{
		"name":      pod.Name,
		"namespace": pod.Namespace,
		"uid":       pod.UID,
	}
	spec := map[string]interface{}{
		"serviceAccountName": pod.ServiceAccount,
		"nodeName":           pod.NodeName,
		"hostNetwork":        pod.HostNetwork,
		"hostPID":            pod.HostPID,
		"hostIPC":            pod.HostIPC,
	}
	if pod.AutomountServiceAccountToken != nil {
		spec["automountServiceAccountToken"] = *pod.AutomountServiceAccountToken
	}
	if pod.Containers != "" {
		var c []interface{}
		if err := json.Unmarshal([]byte(pod.Containers), &c); err == nil && len(c) > 0 {
			spec["containers"] = c
		}
	}
	if pod.Volumes != "" {
		var v []interface{}
		if err := json.Unmarshal([]byte(pod.Volumes), &v); err == nil && len(v) > 0 {
			spec["volumes"] = v
		}
	}
	if pod.Tolerations != "" {
		var t []interface{}
		if err := json.Unmarshal([]byte(pod.Tolerations), &t); err == nil && len(t) > 0 {
			spec["tolerations"] = t
		}
	}
	if pod.Affinity != "" {
		var a interface{}
		if err := json.Unmarshal([]byte(pod.Affinity), &a); err == nil {
			spec["affinity"] = a
		}
	}
	obj := podSpecForYAML{
		APIVersion: "v1",
		Kind:       "Pod",
		Metadata:   meta,
		Spec:       spec,
	}
	return yaml.Marshal(obj)
}

// GetPodSpecYAML returns the pod specification as YAML by numeric ID.
func GetPodSpecYAML(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var pod models.Pod
		if err := db.First(&pod, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Pod not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		out, err := buildPodSpecYAML(&pod)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.Header("Content-Type", "application/x-yaml")
		if c.Query("download") == "1" {
			c.Header("Content-Disposition", `attachment; filename="pod-`+pod.Name+`.yaml"`)
		}
		c.Data(http.StatusOK, "application/x-yaml", out)
	}
}

// GetPodSpecYAMLByUID returns the pod specification as YAML by UID.
func GetPodSpecYAMLByUID(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		podUID := c.Param("uid")
		if podUID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "podUid is required"})
			return
		}
		var pod models.Pod
		if err := db.Where("uid = ?", podUID).First(&pod).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Pod not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		out, err := buildPodSpecYAML(&pod)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.Header("Content-Type", "application/x-yaml")
		if c.Query("download") == "1" {
			c.Header("Content-Disposition", `attachment; filename="pod-`+pod.Name+`.yaml"`)
		}
		c.Data(http.StatusOK, "application/x-yaml", out)
	}
}
