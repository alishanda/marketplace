package queue

type Delivery struct {
	ch chan string
}

func NewDelivery(size int) *Delivery {
	return &Delivery{ch: make(chan string, size)}
}

func (d *Delivery) Enqueue(orderID string) {
	select {
	case d.ch <- orderID:
	default:
	}
}

func (d *Delivery) Jobs() <-chan string {
	return d.ch
}
