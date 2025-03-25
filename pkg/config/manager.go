package config

import (
	"context"
	"fmt"
	"os"
	"sync"

	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	// ConfigMapName is the name of the ConfigMap containing the configuration
	ConfigMapName = "replicas-controller-config"
	// DefaultConfigMapNamespace is the default namespace of the ConfigMap
	DefaultConfigMapNamespace = "kubedynamicscaler-system"
	// ConfigMapKey is the key in the ConfigMap containing the configuration
	ConfigMapKey = "config.yaml"
	// EnvConfigNamespace is the environment variable to override the ConfigMap namespace
	EnvConfigNamespace = "CONFIG_NAMESPACE"
)

// Manager manages the global configuration
type Manager struct {
	client    client.Client
	config    *GlobalConfig
	namespace string
	mutex     sync.RWMutex
}

// NewManager creates a new configuration manager
func NewManager(client client.Client) *Manager {
	namespace := os.Getenv(EnvConfigNamespace)
	if namespace == "" {
		namespace = DefaultConfigMapNamespace
	}
	log := log.Log.WithName("config.Manager")
	log.Info("Creating new ConfigManager", "namespace", namespace)
	return &Manager{
		client:    client,
		config:    DefaultConfig(),
		namespace: namespace,
	}
}

// SetupWithManager sets up the manager with the Manager.
func (m *Manager) SetupWithManager(mgr manager.Manager) error {
	log := log.Log.WithName("config.Manager")
	log.Info("Setting up ConfigManager with Manager")
	// Create a new controller for watching ConfigMap changes
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.ConfigMap{}).
		WithEventFilter(predicate.And(
			predicate.NewPredicateFuncs(func(obj client.Object) bool {
				// Only watch our specific ConfigMap in our namespace
				match := obj.GetName() == ConfigMapName && obj.GetNamespace() == m.namespace
				log.Info("ConfigMap filter", "name", obj.GetName(), "namespace", obj.GetNamespace(), "match", match)
				return match
			}),
			// Only watch ConfigMaps in our namespace
			predicate.NewPredicateFuncs(func(obj client.Object) bool {
				match := obj.GetNamespace() == m.namespace
				log.Info("Namespace filter", "namespace", obj.GetNamespace(), "match", match)
				return match
			}),
		)).
		Complete(m)
}

// Reconcile handles ConfigMap reconciliation
func (m *Manager) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("ConfigMap changed, reloading configuration", "name", req.Name, "namespace", req.Namespace)

	if err := m.loadConfig(ctx); err != nil {
		log.Error(err, "Failed to reload configuration")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// Start starts watching the ConfigMap for changes
func (m *Manager) Start(ctx context.Context) error {
	log := log.FromContext(ctx)
	log.Info("Starting ConfigManager")

	// Initial load of configuration
	if err := m.loadConfig(ctx); err != nil {
		log.Error(err, "Failed to load initial configuration")
		// Don't return error, use default config
	}

	return nil
}

// GetConfig returns the current configuration
func (m *Manager) GetConfig() *GlobalConfig {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	log := log.Log.WithName("config.Manager")
	log.Info("Getting config", "globalPercentage", m.config.GlobalPercentage)
	return m.config
}

// loadConfig loads the configuration from the ConfigMap
func (m *Manager) loadConfig(ctx context.Context) error {
	log := log.FromContext(ctx)
	log.Info("Loading configuration from ConfigMap", "name", ConfigMapName, "namespace", m.namespace)

	// Create a namespaced client
	namespacedClient := client.NewNamespacedClient(m.client, m.namespace)

	cm := &corev1.ConfigMap{}
	err := namespacedClient.Get(ctx, types.NamespacedName{
		Name:      ConfigMapName,
		Namespace: m.namespace,
	}, cm)
	if err != nil {
		return fmt.Errorf("failed to get ConfigMap: %w", err)
	}

	configData, ok := cm.Data[ConfigMapKey]
	if !ok {
		return fmt.Errorf("ConfigMap key %s not found", ConfigMapKey)
	}

	config := &GlobalConfig{}
	if err := yaml.Unmarshal([]byte(configData), config); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.config = config

	// Log the loaded configuration
	log.Info("Configuration loaded successfully",
		"globalPercentage", config.GlobalPercentage,
		"maxReplicas", config.MaxReplicas,
		"minReplicas", config.MinReplicas)

	return nil
}

// RefreshConfig forces a refresh of the configuration
func (m *Manager) RefreshConfig(ctx context.Context) error {
	return m.loadConfig(ctx)
}
