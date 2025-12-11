package types

type VendorConfig struct {
	Vendors []VendorSpec `yaml:"vendors"`
}

type VendorSpec struct {
	Name    string       `yaml:"name"`
	URL     string       `yaml:"url"`
	License string       `yaml:"license"`
	Specs   []BranchSpec `yaml:"specs"`
}

type BranchSpec struct {
	Ref           string        `yaml:"ref"`
	DefaultTarget string        `yaml:"default_target,omitempty"`
	Mapping       []PathMapping `yaml:"mapping"`
}

type PathMapping struct {
	From string `yaml:"from"`
	To   string `yaml:"to"`
}

type VendorLock struct {
	Vendors []LockDetails `yaml:"vendors"`
}

type LockDetails struct {
	Name        string `yaml:"name"`
	Ref         string `yaml:"ref"`
	CommitHash  string `yaml:"commit_hash"`
	LicensePath string `yaml:"license_path"` // Automatically managed
	Updated     string `yaml:"updated"`
}

// PathConflict represents a conflict between two vendors mapping to overlapping paths
type PathConflict struct {
	Path     string
	Vendor1  string
	Vendor2  string
	Mapping1 PathMapping
	Mapping2 PathMapping
}