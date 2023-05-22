package uow

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/go-sql-driver/mysql"
)

// Test types
type Product struct {
	id     uuid.UUID
	amount uint32
}

func NewProduct(amount uint32) *Product {
	return &Product{
		id:     uuid.New(),
		amount: amount,
	}
}

type Order struct {
	id        uuid.UUID
	productId uuid.UUID
	amount    uint32
}

func NewOrder(productId uuid.UUID, amount uint32) *Order {
	return &Order{
		id:        uuid.New(),
		productId: productId,
		amount:    amount,
	}
}

// Test repositories
type ProductRepository struct {
	tx *sql.Tx
}

func NewProductRepository(tx *sql.Tx) *ProductRepository {
	return &ProductRepository{tx: tx}
}

func (r *ProductRepository) Get(ctx context.Context, id uuid.UUID) (*Product, error) {
	stmt, err := r.tx.PrepareContext(ctx, "SELECT id, amount FROM products WHERE id = ?")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	var product Product
	err = stmt.QueryRowContext(ctx, id).Scan(&product.id, &product.amount)
	if err != nil {
		return nil, err
	}

	return &product, nil
}

func (r *ProductRepository) Save(ctx context.Context, product *Product) error {
	stmt, err := r.tx.PrepareContext(ctx, "INSERT INTO products (id, amount) VALUES (?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.ExecContext(ctx, product.id, product.amount)
	return err
}

func (r *ProductRepository) Update(ctx context.Context, product *Product) error {
	stmt, err := r.tx.PrepareContext(ctx, "UPDATE products SET amount = ? WHERE id = ?")
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.ExecContext(ctx, product.amount, product.id)
	return err
}

type OrderRepository struct {
	tx *sql.Tx
}

func NewOrderRepository(tx *sql.Tx) *OrderRepository {
	return &OrderRepository{tx: tx}
}

func (r *OrderRepository) Get(ctx context.Context, id uuid.UUID) (*Order, error) {
	stmt, err := r.tx.PrepareContext(ctx, "SELECT id, product_id, amount FROM orders WHERE id = ?")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	var order Order
	err = stmt.QueryRowContext(ctx, id).Scan(&order.id, &order.productId, &order.amount)
	if err != nil {
		return nil, err
	}

	return &order, nil
}

func (r *OrderRepository) Save(ctx context.Context, order *Order) error {
	stmt, err := r.tx.PrepareContext(ctx, "INSERT INTO orders (id, product_id, amount) VALUES (?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.ExecContext(ctx, order.id, order.productId, order.amount)
	return err
}

// Tests
func Test_Transaction_NewTransaction(t *testing.T) {
	tx := &sql.Tx{}
	repositories := make(map[RepositoryName]RepositoryFactory)

	transaction := NewTransaction(tx, repositories)
	assert.NotNil(t, transaction)
	assert.Same(t, tx, transaction.tx)
	assert.Equal(t, repositories, transaction.repositories)
}

func Test_Transaction_Get(t *testing.T) {
	tx := &sql.Tx{}
	repositories := make(map[RepositoryName]RepositoryFactory)

	transaction := NewTransaction(tx, repositories)

	_, err := transaction.Get("ProductRepository")
	assert.ErrorIs(t, ErrRepositoryNotRegistered, err)

	_, err = transaction.Get("OrderRepository")
	assert.ErrorIs(t, ErrRepositoryNotRegistered, err)

	transaction.repositories["ProductRepository"] = func(tx *sql.Tx) Repository {
		return NewProductRepository(tx)
	}

	transaction.repositories["OrderRepository"] = func(tx *sql.Tx) Repository {
		return NewOrderRepository(tx)
	}

	productRepository, err := transaction.Get("ProductRepository")
	assert.Nil(t, err)
	assert.IsType(t, &ProductRepository{}, productRepository)
	assert.Same(t, tx, productRepository.(*ProductRepository).tx)

	orderRepository, err := transaction.Get("OrderRepository")
	assert.Nil(t, err)
	assert.IsType(t, &OrderRepository{}, orderRepository)
	assert.Same(t, tx, orderRepository.(*OrderRepository).tx)
}

func Test_UnitOfWork_NewUnitOfWork(t *testing.T) {
	db := &sql.DB{}

	uow := NewUnitOfWork(db)
	assert.NotNil(t, uow)
	assert.Same(t, db, uow.db)
	assert.NotNil(t, uow.repositories)
}

func Test_UnitOfWork_Register(t *testing.T) {
	productRepositoryFactory := func(tx *sql.Tx) Repository {
		return NewProductRepository(tx)
	}

	orderRepositoryFactory := func(tx *sql.Tx) Repository {
		return NewOrderRepository(tx)
	}

	uow := NewUnitOfWork(&sql.DB{})

	err := uow.Register("ProductRepository", productRepositoryFactory)
	assert.Nil(t, err)

	err = uow.Register("OrderRepository", orderRepositoryFactory)
	assert.Nil(t, err)

	err = uow.Register("ProductRepository", productRepositoryFactory)
	assert.ErrorIs(t, ErrRepositoryAlreadyRegistered, err)

	err = uow.Register("OrderRepository", orderRepositoryFactory)
	assert.ErrorIs(t, ErrRepositoryAlreadyRegistered, err)

	factory, ok := uow.repositories["ProductRepository"]
	assert.True(t, ok)
	assert.NotNil(t, factory)
	assert.IsType(t, &ProductRepository{}, factory(&sql.Tx{}).(*ProductRepository))

	factory, ok = uow.repositories["OrderRepository"]
	assert.True(t, ok)
	assert.NotNil(t, factory)
	assert.IsType(t, &OrderRepository{}, factory(&sql.Tx{}).(*OrderRepository))
}

func Test_UnitOfWork_Remove(t *testing.T) {
	uow := NewUnitOfWork(&sql.DB{})

	err := uow.Remove("ProductRepository")
	assert.ErrorIs(t, err, ErrRepositoryNotRegistered)

	err = uow.Remove("OrderRepository")
	assert.ErrorIs(t, err, ErrRepositoryNotRegistered)

	uow.repositories["ProductRepository"] = func(tx *sql.Tx) Repository {
		return NewProductRepository(tx)
	}

	uow.repositories["OrderRepository"] = func(tx *sql.Tx) Repository {
		return NewOrderRepository(tx)
	}

	err = uow.Remove("ProductRepository")
	assert.Nil(t, err)

	err = uow.Remove("OrderRepository")
	assert.Nil(t, err)

	assert.Equal(t, 0, len(uow.repositories))
}

func Test_UnitOfWork_Has(t *testing.T) {
	uow := NewUnitOfWork(&sql.DB{})

	has := uow.Has("ProductRepository")
	assert.False(t, has)

	has = uow.Has("OrderRepository")
	assert.False(t, has)

	uow.repositories["ProductRepository"] = func(tx *sql.Tx) Repository {
		return NewProductRepository(tx)
	}

	uow.repositories["OrderRepository"] = func(tx *sql.Tx) Repository {
		return NewOrderRepository(tx)
	}

	has = uow.Has("ProductRepository")
	assert.True(t, has)

	has = uow.Has("OrderRepository")
	assert.True(t, has)
}

func Test_UnitOfWork_Clear(t *testing.T) {
	uow := NewUnitOfWork(&sql.DB{})

	uow.repositories["ProductRepository"] = func(tx *sql.Tx) Repository {
		return NewProductRepository(tx)
	}

	uow.repositories["OrderRepository"] = func(tx *sql.Tx) Repository {
		return NewOrderRepository(tx)
	}

	uow.Clear()

	assert.NotNil(t, uow.repositories)
	assert.Equal(t, 0, len(uow.repositories))
}

func Test_UnitOfWork_Do_WhenTransactionSucceeds(t *testing.T) {
	// Connect to db and prepare tables
	db, err := sql.Open("mysql", "user:user@tcp(mysql:3306)/test")
	require.Nil(t, err)
	defer db.Close()

	_, err = db.Exec("DROP TABLE IF EXISTS orders")
	require.Nil(t, err)

	_, err = db.Exec("DROP TABLE IF EXISTS products")
	require.Nil(t, err)

	_, err = db.Exec(`
		CREATE TABLE products (
			id VARCHAR(36) PRIMARY KEY, 
			amount INT(32) UNSIGNED NOT NULL
	  	);
	`)
	require.Nil(t, err)

	_, err = db.Exec(`
		CREATE TABLE orders (
			id VARCHAR(36) PRIMARY KEY, 
			product_id VARCHAR(36) NOT NULL, 
			amount INT(32) UNSIGNED NOT NULL,
			FOREIGN KEY (product_id) REFERENCES products(id)
	  	);
	`)
	require.Nil(t, err)

	// Create uow and register repositories
	ctx := context.Background()
	uow := NewUnitOfWork(db)

	uow.Register("ProductRepository", func(tx *sql.Tx) Repository {
		return NewProductRepository(tx)
	})

	uow.Register("OrderRepository", func(tx *sql.Tx) Repository {
		return NewOrderRepository(tx)
	})

	// Save product, amount = 10
	product := NewProduct(10)

	err = uow.Do(ctx, func(ctx context.Context, tx TX) error {
		// Get repository
		repository, err := tx.Get("ProductRepository")
		if err != nil {
			return err
		}

		productRepository, ok := repository.(*ProductRepository)
		if !ok {
			return errors.New("invalid type")
		}

		// Save product
		err = productRepository.Save(ctx, product)
		return err
	})
	assert.Nil(t, err)

	// Save order, amount = 3
	order := NewOrder(product.id, 3)

	err = uow.Do(ctx, func(ctx context.Context, tx TX) error {
		// Get repositories
		repository, err := tx.Get("ProductRepository")
		if err != nil {
			return err
		}

		productRepository, ok := repository.(*ProductRepository)
		if !ok {
			return errors.New("invalid type")
		}

		repository, err = tx.Get("OrderRepository")
		if err != nil {
			return err
		}

		orderRepository, ok := repository.(*OrderRepository)
		if !ok {
			return errors.New("invalid type")
		}

		// Get itens
		productSaved, err := productRepository.Get(ctx, order.productId)
		if err != nil {
			return err
		}

		if productSaved.amount != 10 {
			return errors.New("product saved amount must be 10")
		}

		// Update product
		productSaved.amount -= order.amount

		err = productRepository.Update(ctx, productSaved)
		if err != nil {
			return err
		}

		// Save order
		err = orderRepository.Save(ctx, order)
		return err
	})
	assert.Nil(t, err)

	// Verify amounts
	err = uow.Do(ctx, func(ctx context.Context, tx TX) error {
		// Get repositories
		repository, err := tx.Get("ProductRepository")
		if err != nil {
			return err
		}

		productRepository, ok := repository.(*ProductRepository)
		if !ok {
			return errors.New("invalid type")
		}

		repository, err = tx.Get("OrderRepository")
		if err != nil {
			return err
		}

		orderRepository, ok := repository.(*OrderRepository)
		if !ok {
			return errors.New("invalid type")
		}

		// Get itens
		productSaved, err := productRepository.Get(ctx, product.id)
		if err != nil {
			return err
		}

		orderSaved, err := orderRepository.Get(ctx, order.id)
		if err != nil {
			return err
		}

		// Verify amounts
		if productSaved.amount != 7 {
			return errors.New("product saved amount must be 7")
		}

		if orderSaved.amount != 3 {
			return errors.New("order saved amount must be 3")
		}

		return nil
	})
	assert.Nil(t, err)
}

func Test_UnitOfWork_Do_WhenTransactionFails(t *testing.T) {
	// Connect to db and prepare tables
	db, err := sql.Open("mysql", "user:user@tcp(mysql:3306)/test")
	require.Nil(t, err)
	defer db.Close()

	_, err = db.Exec("DROP TABLE IF EXISTS orders")
	require.Nil(t, err)

	_, err = db.Exec("DROP TABLE IF EXISTS products")
	require.Nil(t, err)

	_, err = db.Exec(`
		CREATE TABLE products (
			id VARCHAR(36) PRIMARY KEY, 
			amount INT(32) UNSIGNED NOT NULL
	  	);
	`)
	require.Nil(t, err)

	_, err = db.Exec(`
		CREATE TABLE orders (
			id VARCHAR(36) PRIMARY KEY, 
			product_id VARCHAR(36) NOT NULL, 
			amount INT(32) UNSIGNED NOT NULL,
			FOREIGN KEY (product_id) REFERENCES products(id)
	  	);
	`)
	require.Nil(t, err)

	// Create uow and register repositories
	ctx := context.Background()
	uow := NewUnitOfWork(db)

	uow.Register("ProductRepository", func(tx *sql.Tx) Repository {
		return NewProductRepository(tx)
	})

	uow.Register("OrderRepository", func(tx *sql.Tx) Repository {
		return NewOrderRepository(tx)
	})

	// Save product, amount = 10
	product := NewProduct(10)

	err = uow.Do(ctx, func(ctx context.Context, tx TX) error {
		// Get repository
		repository, err := tx.Get("ProductRepository")
		if err != nil {
			return err
		}

		productRepository, ok := repository.(*ProductRepository)
		if !ok {
			return errors.New("invalid type")
		}

		// Save product
		err = productRepository.Save(ctx, product)
		return err
	})
	assert.Nil(t, err)

	// Try to save order, amount = 3
	order := NewOrder(product.id, 3)

	err = uow.Do(ctx, func(ctx context.Context, tx TX) error {
		// Get repositories
		repository, err := tx.Get("ProductRepository")
		if err != nil {
			return err
		}

		productRepository, ok := repository.(*ProductRepository)
		if !ok {
			return errors.New("invalid type")
		}

		repository, err = tx.Get("OrderRepository")
		if err != nil {
			return err
		}

		orderRepository, ok := repository.(*OrderRepository)
		if !ok {
			return errors.New("invalid type")
		}

		// Get itens
		productSaved, err := productRepository.Get(ctx, order.productId)
		if err != nil {
			return err
		}

		if productSaved.amount != 10 {
			return errors.New("product saved amount must be 10")
		}

		// Update product
		productSaved.amount -= order.amount

		err = productRepository.Update(ctx, productSaved)
		if err != nil {
			return err
		}

		// Add a bug (an id of a nonexistent product)
		order.productId = uuid.New()

		// Try to save order
		err = orderRepository.Save(ctx, order)
		return err
	})
	assert.NotNil(t, err)

	// Verify amounts
	err = uow.Do(ctx, func(ctx context.Context, tx TX) error {
		// Get Repositories
		repository, err := tx.Get("ProductRepository")
		if err != nil {
			return err
		}

		productRepository, ok := repository.(*ProductRepository)
		if !ok {
			return errors.New("invalid type")
		}

		repository, err = tx.Get("OrderRepository")
		if err != nil {
			return err
		}

		orderRepository, ok := repository.(*OrderRepository)
		if !ok {
			return errors.New("invalid type")
		}

		// Get itens
		productSaved, err := productRepository.Get(ctx, product.id)
		if err != nil {
			return err
		}

		_, err = orderRepository.Get(ctx, order.id)
		if err == nil {
			return errors.New("the order should not be saved")
		}

		// Verify amounts
		if productSaved.amount != 10 {
			return errors.New("product saved amount must be 10")
		}

		return nil
	})
	assert.Nil(t, err)
}
