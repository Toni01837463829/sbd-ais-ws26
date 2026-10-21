package repository

import (
	"ordersystem/model"
	"time"
)

type DatabaseHandler struct {
	// drinks represent all available drinks
	drinks []model.Drink
	// orders serves as order history
	orders []model.Order
}

// todo
func NewDatabaseHandler() *DatabaseHandler {
	// Init the drinks slice with some test data
	// drinks := ...
	drinks := []model.Drink{
		{ID: 1, Name: "Pina Colada", Price: 7.50, Description: "Rum, coconut cream and pineapple juice"},
		{ID: 2, Name: "Blue Lagoon", Price: 7.00, Description: "Vodka, blue curacao and lemonade"},
		{ID: 3, Name: "Mojito", Price: 6.50, Description: "Rum, mint, lime and soda"},
		{ID: 4, Name: "Sex on the Beach", Price: 7.50, Description: "Vodka, peach schnapps, orange and cranberry juice"},
		{ID: 5, Name: "Mai Tai", Price: 8.00, Description: "Rum, orange liqueur, lime and almond syrup"},
	}

	// Init orders slice with some test data
	now := time.Now()
	orders := []model.Order{
		{DrinkID: 1, CreatedAt: now.Add(-3 * time.Hour), Amount: 2},
		{DrinkID: 2, CreatedAt: now.Add(-2 * time.Hour), Amount: 1},
		{DrinkID: 3, CreatedAt: now.Add(-1 * time.Hour), Amount: 4},
		{DrinkID: 5, CreatedAt: now.Add(-30 * time.Minute), Amount: 3},
	}

	return &DatabaseHandler{
		drinks: drinks,
		orders: orders,
	}
}

func (db *DatabaseHandler) GetDrinks() []model.Drink {
	return db.drinks
}

func (db *DatabaseHandler) GetOrders() []model.Order {
	return append([]model.Order{}, db.orders...)
}

// todo
func (db *DatabaseHandler) GetTotalledOrders() map[uint64]uint64 {
	// calculate total orders
	// key = DrinkID, value = Amount of orders
	// totalledOrders map[uint64]uint64
	totalledOrders := make(map[uint64]uint64)
	for _, order := range db.orders {
		totalledOrders[order.DrinkID] += order.Amount
	}
	return totalledOrders
}

func (db *DatabaseHandler) AddOrder(order *model.Order) {
	// todo
	// add order to db.orders slice
	order.CreatedAt = time.Now()
	db.orders = append(db.orders, *order)
}
