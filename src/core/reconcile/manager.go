package reconcile

import (
	"context"
	"fmt"
	"sync"

	"github.com/cloud-barista/cb-tumblebug/src/core/model"
)

// Reconciler is an interface that every resource-specific reconciler must implement.
type Reconciler interface {
	Reconcile(ctx context.Context, nsId string, resourceId string, optPreloadedStatus *model.CspResourceStatusResponse) (any, error)
	ReconcileAll(ctx context.Context, nsId string, maxConcurrent int) (model.ResourceReconcileResults, error)
}

// Manager is a singleton registry that holds and routes reconcile requests to appropriate Reconcilers.
type Manager struct {
	reconcilers map[string]Reconciler
	mux         sync.RWMutex
}

var instance *Manager
var once sync.Once

// GetManager returns the singleton instance of the Manager.
func GetManager() *Manager {
	once.Do(func() {
		instance = &Manager{
			reconcilers: make(map[string]Reconciler),
		}
	})
	return instance
}

// RegisterReconciler registers a specific Reconciler for a given resourceType.
func (m *Manager) RegisterReconciler(resourceType string, reconciler Reconciler) {
	m.mux.Lock()
	defer m.mux.Unlock()
	m.reconcilers[resourceType] = reconciler
}

// RunReconcile routes the reconcile request to the registered Reconciler based on resourceType.
func (m *Manager) RunReconcile(ctx context.Context, nsId string, resourceType string, resourceId string, optPreloadedStatus *model.CspResourceStatusResponse) (any, error) {
	m.mux.RLock()
	reconciler, exists := m.reconcilers[resourceType]
	m.mux.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no reconciler registered for resource type: %s", resourceType)
	}

	return reconciler.Reconcile(ctx, nsId, resourceId, optPreloadedStatus)
}

// RunReconcileAll routes the namespace-level batch reconcile request to the registered Reconciler.
func (m *Manager) RunReconcileAll(ctx context.Context, nsId string, resourceType string, maxConcurrent int) (model.ResourceReconcileResults, error) {
	m.mux.RLock()
	reconciler, exists := m.reconcilers[resourceType]
	m.mux.RUnlock()

	if !exists {
		return model.ResourceReconcileResults{}, fmt.Errorf("no reconciler registered for resource type: %s", resourceType)
	}

	return reconciler.ReconcileAll(ctx, nsId, maxConcurrent)
}
