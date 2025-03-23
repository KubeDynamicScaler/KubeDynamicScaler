package config

// GlobalConfig represents the global configuration for the controller
type GlobalConfig struct {
	// GlobalPercentage is the default percentage to scale replicas
	GlobalPercentage int32 `yaml:"globalPercentage"`
	// MaxReplicas is the maximum number of replicas allowed
	MaxReplicas int32 `yaml:"maxReplicas"`
	// MinReplicas is the minimum number of replicas allowed
	MinReplicas int32 `yaml:"minReplicas"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *GlobalConfig {
	return &GlobalConfig{
		GlobalPercentage: 100,
		MaxReplicas:      100,
		MinReplicas:      1,
	}
}
