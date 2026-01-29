package steps

import (
	"sync"
)

// MockDatabaseClient implements DatabaseClient for testing
type MockDatabaseClient struct {
	mu           sync.Mutex
	databases    map[string]bool
	createCalls  []string
	dropCalls    []string
	listCalls    []string
	pingError    error
	createError  error
	dropError    error
	listError    error
	existsOnCall int
	callCount    int
}

// NewMockDatabaseClient creates a new mock database client
func NewMockDatabaseClient() *MockDatabaseClient {
	return &MockDatabaseClient{
		databases:   make(map[string]bool),
		createCalls: make([]string, 0),
		dropCalls:   make([]string, 0),
		listCalls:   make([]string, 0),
	}
}

func (m *MockDatabaseClient) Ping() error {
	return m.pingError
}

func (m *MockDatabaseClient) Close() error {
	return nil
}

func (m *MockDatabaseClient) CreateDatabase(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.createCalls = append(m.createCalls, name)
	m.callCount++

	if m.createError != nil {
		return m.createError
	}

	if m.existsOnCall > 0 && m.callCount <= m.existsOnCall {
		return &DatabaseExistsError{Name: name}
	}

	if m.databases[name] {
		return &DatabaseExistsError{Name: name}
	}

	m.databases[name] = true
	return nil
}

func (m *MockDatabaseClient) DropDatabase(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.dropCalls = append(m.dropCalls, name)

	if m.dropError != nil {
		return m.dropError
	}

	delete(m.databases, name)
	return nil
}

func (m *MockDatabaseClient) ListDatabases(pattern string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.listCalls = append(m.listCalls, pattern)

	if m.listError != nil {
		return nil, m.listError
	}

	var result []string
	for name := range m.databases {
		result = append(result, name)
	}
	return result, nil
}

func (m *MockDatabaseClient) SetPingError(err error) {
	m.pingError = err
}

func (m *MockDatabaseClient) SetCreateError(err error) {
	m.createError = err
}

func (m *MockDatabaseClient) SetDropError(err error) {
	m.dropError = err
}

func (m *MockDatabaseClient) SetListError(err error) {
	m.listError = err
}

func (m *MockDatabaseClient) SetExistsOnFirstNCalls(n int) {
	m.existsOnCall = n
}

func (m *MockDatabaseClient) AddDatabase(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.databases[name] = true
}

func (m *MockDatabaseClient) GetCreateCalls() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]string, len(m.createCalls))
	copy(result, m.createCalls)
	return result
}

func (m *MockDatabaseClient) GetDropCalls() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]string, len(m.dropCalls))
	copy(result, m.dropCalls)
	return result
}

func (m *MockDatabaseClient) HasDatabase(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.databases[name]
}

func (m *MockDatabaseClient) DatabaseCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.databases)
}

// MockClientFactory creates a factory that returns the provided mock client
func MockClientFactory(client *MockDatabaseClient) DatabaseClientFactory {
	return func(engine string, opts DatabaseOptions) (DatabaseClient, error) {
		return client, nil
	}
}
