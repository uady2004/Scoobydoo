package events

import "time"

const TopicOrderCreated = "order.created"

type OrderCreated struct {
	EventID      string    `json:"event_id"`
	OccurredAt   time.Time `json:"occurred_at"`
	OrderID      string    `json:"order_id"`
	BuyerID      string    `json:"buyer_id"`
	BuyerEmail   string    `json:"buyer_email"`
	BuyerName    string    `json:"buyer_name"`
	SellerID     string    `json:"seller_id"`
	ProductID    string    `json:"product_id"`
	ProductName  string    `json:"product_name"`
	Quantity     int       `json:"quantity"`
	TotalAmount  float64   `json:"total_amount"`
	Currency     string    `json:"currency"`
	SupportEmail string    `json:"support_email"`
}
