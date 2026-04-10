package config

type keychainStore struct {
	get    func(service, user string) (string, error)
	set    func(service, user, password string) error
	delete func(service, user string) error
}

func NewKeychainStore(
	getFn func(service, user string) (string, error),
	setFn func(service, user, password string) error,
	deleteFn func(service, user string) error,
) KeychainStore {
	return &keychainStore{
		get:    getFn,
		set:    setFn,
		delete: deleteFn,
	}
}

func (k *keychainStore) Get(service, user string) (string, error) {
	return k.get(service, user)
}

func (k *keychainStore) Set(service, user, password string) error {
	return k.set(service, user, password)
}

func (k *keychainStore) Delete(service, user string) error {
	return k.delete(service, user)
}
