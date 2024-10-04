package migration

import (
	"context"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

const ConfigMapName = "admigration-config"

type Status string

const (
	StatusUnknown Status = "unknown"
	StatusRunning Status = "running"
	StatusDone    Status = "done"
)

type Action string

const (
	ActionCheck    Action = "check"
	ActionMigrate  Action = "migrate"
	ActionRollback Action = "rollback"
)

type Configuration struct {
	Enabled bool
	Status  Status
	Action  Action
	Limit   int
	Users   []string
}

func NewDefaultConfiguration() *Configuration {
	return &Configuration{
		Enabled: true,
		Status:  StatusUnknown,
		Action:  ActionMigrate,
		Limit:   1000,
	}
}

func GetOrCreateConfig(ctx context.Context, configMapInterface typedcorev1.ConfigMapInterface) (*Configuration, error) {
	configuration := NewDefaultConfiguration()

	var cm *corev1.ConfigMap
	var err error

	cm, err = configMapInterface.Get(ctx, ConfigMapName, v1.GetOptions{})
	if err != nil {
		if !apierror.IsNotFound(err) {
			return nil, err
		}

		// if not found create and store the default map
		cm, err = configMapInterface.Create(ctx, &corev1.ConfigMap{
			ObjectMeta: v1.ObjectMeta{Name: ConfigMapName},
			Data:       convertConfigurationToConfigMap(configuration),
		}, v1.CreateOptions{})
		if err != nil {
			return nil, err
		}
	}

	configuration = convertConfigMapToConfiguration(cm.Data)

	return configuration, nil
}

func convertConfigMapToConfiguration(m map[string]string) *Configuration {
	configuration := NewDefaultConfiguration()

	if enabledStr, found := m["enabled"]; found {
		enabled, err := strconv.ParseBool(enabledStr)
		if err != nil {
			// log
		} else {
			configuration.Enabled = enabled
		}
	}

	if statusStr, found := m["status"]; found {
		status := Status(statusStr)
		switch status {
		case StatusRunning, StatusDone:
			configuration.Status = status
		}
	}

	if actionStr, found := m["action"]; found {
		action := Action(actionStr)
		switch action {
		case ActionCheck, ActionMigrate, ActionRollback:
			configuration.Action = action
		}
	}

	if limitStr, found := m["limit"]; found {
		limit, err := strconv.Atoi(limitStr)
		if err != nil {
			// log
		} else {
			configuration.Limit = limit
		}
	}

	return configuration
}

func convertConfigurationToConfigMap(config *Configuration) map[string]string {
	data := map[string]string{
		"limit":  strconv.Itoa(config.Limit),
		"status": string(config.Status),
		"action": string(config.Action),
	}

	return data
}
