package core

import (
	"testing"
)

func TestHookInterfaces(t *testing.T) {
	m := &TestModelWithHooks{}

	// Test all hook interfaces
	var _ AfterCreater = m
	var _ BeforeUpdater = m
	var _ AfterUpdater = m
	var _ BeforeDeleter = m
	var _ AfterDeleter = m
	var _ BeforeSaver = m
	var _ AfterSaver = m
}

func TestBeforeSaver_Success(t *testing.T) {
	m := &TestModelWithHooks{}
	if err := m.BeforeSave(); err != nil {
		t.Errorf("BeforeSave() error = %v", err)
	}
	if !m.beforeSaveCalled {
		t.Error("BeforeSave was not called")
	}
}

func TestBeforeSaver_Error(t *testing.T) {
	m := &TestModelWithHooks{hookErr: errHookFail}
	if err := m.BeforeSave(); err != errHookFail {
		t.Errorf("BeforeSave() error = %v, want %v", err, errHookFail)
	}
}

func TestAfterSaver_Success(t *testing.T) {
	m := &TestModelWithHooks{}
	if err := m.AfterSave(); err != nil {
		t.Errorf("AfterSave() error = %v", err)
	}
	if !m.afterSaveCalled {
		t.Error("AfterSave was not called")
	}
}

func TestAfterCreater_Success(t *testing.T) {
	m := &TestModelWithHooks{}
	if err := m.AfterCreate(); err != nil {
		t.Errorf("AfterCreate() error = %v", err)
	}
	if !m.afterCreateCalled {
		t.Error("AfterCreate was not called")
	}
}

func TestBeforeUpdater_Success(t *testing.T) {
	m := &TestModelWithHooks{}
	if err := m.BeforeUpdate(); err != nil {
		t.Errorf("BeforeUpdate() error = %v", err)
	}
	if !m.beforeUpdateCalled {
		t.Error("BeforeUpdate was not called")
	}
}

func TestAfterUpdater_Success(t *testing.T) {
	m := &TestModelWithHooks{}
	if err := m.AfterUpdate(); err != nil {
		t.Errorf("AfterUpdate() error = %v", err)
	}
	if !m.afterUpdateCalled {
		t.Error("AfterUpdate was not called")
	}
}

func TestBeforeDeleter_Success(t *testing.T) {
	m := &TestModelWithHooks{}
	if err := m.BeforeDelete(); err != nil {
		t.Errorf("BeforeDelete() error = %v", err)
	}
	if !m.beforeDeleteCalled {
		t.Error("BeforeDelete was not called")
	}
}

func TestAfterDeleter_Success(t *testing.T) {
	m := &TestModelWithHooks{}
	if err := m.AfterDelete(); err != nil {
		t.Errorf("AfterDelete() error = %v", err)
	}
	if !m.afterDeleteCalled {
		t.Error("AfterDelete was not called")
	}
}

func TestAllHooks_WithError(t *testing.T) {
	hooks := []struct {
		name string
		fn   func(*TestModelWithHooks) error
	}{
		{"BeforeSave", func(m *TestModelWithHooks) error { return m.BeforeSave() }},
		{"AfterSave", func(m *TestModelWithHooks) error { return m.AfterSave() }},
		{"BeforeCreate", func(m *TestModelWithHooks) error { return m.BeforeCreate() }},
		{"AfterCreate", func(m *TestModelWithHooks) error { return m.AfterCreate() }},
		{"BeforeUpdate", func(m *TestModelWithHooks) error { return m.BeforeUpdate() }},
		{"AfterUpdate", func(m *TestModelWithHooks) error { return m.AfterUpdate() }},
		{"BeforeDelete", func(m *TestModelWithHooks) error { return m.BeforeDelete() }},
		{"AfterDelete", func(m *TestModelWithHooks) error { return m.AfterDelete() }},
	}

	for _, h := range hooks {
		t.Run(h.name+"_Error", func(t *testing.T) {
			m := &TestModelWithHooks{hookErr: errHookFail}
			if err := h.fn(m); err != errHookFail {
				t.Errorf("%s error = %v, want %v", h.name, err, errHookFail)
			}
		})
	}
}
