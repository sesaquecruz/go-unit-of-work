# Unit of Work

This project implements a **Unit of Work (UOW)** pattern in Go, providing a convenient way to encapsulate a set of database operations within a single transaction when the **Repository** pattern is used. 

The Unit of Work allows to register repositories and perform operations on them, ensuring that all actions are executed within the same transaction.

## Installation

To install this package, use the go get command:

```
go get github.com/sesaquecruz/go-unit-of-work/uow
```

## Usage

To use the UOW, follow these steps:

1. Import the package into the Go file:

```
import "github.com/sesaquecruz/go-unit-of-work/uow"
```

2. Create an instance passing a db connection (*sql.DB):

```
UOW := uow.NewUnitOfWork(db)
```

3. Register the repositories using the `Register` method:

```
UOW.Register("ProductRepository", func(tx *sql.Tx) Repository {
		return NewProductRepository(tx)
})
```

```
UOW.Register("OrderRepository", func(tx *sql.Tx) Repository {
		return NewOrderRepository(tx)
})
```

4. Perform database operations within a single transaction by calling the `Do` method:

```
err = UOW.Do(ctx, func(ctx context.Context, tx uow.TX) error {
		// Get repositories
		// ...

		// Get product repository
		repository, err := tx.Get("ProductRepository")
		if err != nil {
			return err
		}

		productRepository, ok := repository.(*ProductRepository)
		if !ok {
			return errors.New("invalid repository type")
		}

		// Get order repository
		repository, err = tx.Get("OrderRepository")
		if err != nil {
			return err
		}

		orderRepository, ok := repository.(*OrderRepository)
		if !ok {
			return errors.New("invalid repository type")
		}

		// Execute operations
		// ...
})
```

If an error occurs, the transaction is rolled back and the error is returned. Otherwise, the transaction is committed, and nil is returned.

## Types and Interfaces
The `uow` package provides the following type and interfaces:

```
type RepositoryName string
type Repository any
type RepositoryFactory func(tx *sql.Tx) Repository
```

```
type TX interface {
	Get(name RepositoryName) (Repository, error)
}
```

```
type UOW interface {
	Register(name RepositoryName, factory RepositoryFactory) error
	Remove(name RepositoryName) error
	Has(name RepositoryName) bool
	Clear()
	Do(ctx context.Context, fn func(ctx context.Context, tx TX) error) error
}
```

For more details, see [uow.go](./uow/uow.go).

## Contributing

Contributions to this project are welcome. If you encounter any issues or have ideas for enhancements, feel free to open an issue or submit a pull request.

## License
This project is licensed under the MIT License. Please see the [LICENSE](./LICENSE) file for more details.
