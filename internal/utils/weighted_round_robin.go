package utils

// WeightedItem represents an item with a weight for weighted round-robin selection.
type WeightedItem interface {
	GetWeight() int
	GetCurrentWeight() int
	SetCurrentWeight(int)
}

// SelectByWeightedRoundRobin selects an item using smooth weighted round-robin algorithm.
// The algorithm ensures fair distribution based on weights while maintaining smooth selection order.
//
// Steps:
// 1. Add each item's weight to its current weight
// 2. Select the item with the highest current weight
// 3. Subtract the total weight from the selected item's current weight
//
// This is the Nginx smooth weighted round-robin algorithm.
func SelectByWeightedRoundRobin(items []WeightedItem) WeightedItem {
	if len(items) == 0 {
		return nil
	}
	if len(items) == 1 {
		return items[0]
	}

	totalWeight := 0
	var best WeightedItem

	for _, item := range items {
		totalWeight += item.GetWeight()
		item.SetCurrentWeight(item.GetCurrentWeight() + item.GetWeight())

		if best == nil || item.GetCurrentWeight() > best.GetCurrentWeight() {
			best = item
		}
	}

	if best == nil {
		return items[0]
	}

	best.SetCurrentWeight(best.GetCurrentWeight() - totalWeight)
	return best
}
