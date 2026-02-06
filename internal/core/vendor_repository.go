package core

import (
	"github.com/EmundoT/git-vendor/internal/types"
)

// VendorRepositoryInterface defines the contract for vendor CRUD operations.
// This interface enables mocking in tests and potential alternative storage backends.
type VendorRepositoryInterface interface {
	Find(name string) (*types.VendorSpec, error)
	FindAll() ([]types.VendorSpec, error)
	Exists(name string) (bool, error)
	Save(vendor *types.VendorSpec) error
	Delete(name string) error
	GetConfig() (types.VendorConfig, error)
}

// Compile-time interface satisfaction check.
var _ VendorRepositoryInterface = (*VendorRepository)(nil)

// VendorRepository handles vendor CRUD operations
type VendorRepository struct {
	configStore ConfigStore
}

// NewVendorRepository creates a new VendorRepository
func NewVendorRepository(configStore ConfigStore) *VendorRepository {
	return &VendorRepository{
		configStore: configStore,
	}
}

// Find returns the vendor with the given name, or nil if not found
func (r *VendorRepository) Find(name string) (*types.VendorSpec, error) {
	config, err := r.configStore.Load()
	if err != nil {
		return nil, err
	}

	vendor := FindVendor(config.Vendors, name)
	if vendor == nil {
		return nil, NewVendorNotFoundError(name)
	}

	return vendor, nil
}

// FindAll returns all vendors
func (r *VendorRepository) FindAll() ([]types.VendorSpec, error) {
	config, err := r.configStore.Load()
	if err != nil {
		return nil, err
	}

	return config.Vendors, nil
}

// Exists checks if a vendor with the given name exists
func (r *VendorRepository) Exists(name string) (bool, error) {
	config, err := r.configStore.Load()
	if err != nil {
		return false, err
	}

	return FindVendor(config.Vendors, name) != nil, nil
}

// Save adds or updates a vendor
func (r *VendorRepository) Save(vendor *types.VendorSpec) error {
	config, err := r.configStore.Load()
	if err != nil {
		// If config doesn't exist, create empty one
		config = types.VendorConfig{}
	}

	index := FindVendorIndex(config.Vendors, vendor.Name)
	if index >= 0 {
		// Update existing vendor
		config.Vendors[index] = *vendor
	} else {
		// Add new vendor
		config.Vendors = append(config.Vendors, *vendor)
	}

	return r.configStore.Save(config)
}

// Delete removes a vendor by name
func (r *VendorRepository) Delete(name string) error {
	config, err := r.configStore.Load()
	if err != nil {
		return err
	}

	index := FindVendorIndex(config.Vendors, name)
	if index < 0 {
		return NewVendorNotFoundError(name)
	}

	config.Vendors = append(config.Vendors[:index], config.Vendors[index+1:]...)

	return r.configStore.Save(config)
}

// GetConfig returns the vendor configuration
func (r *VendorRepository) GetConfig() (types.VendorConfig, error) {
	return r.configStore.Load()
}
