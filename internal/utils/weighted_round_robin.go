package utils

// WeightedItem 表示用于加权轮询选择的带权重的项。
type WeightedItem interface {
	GetWeight() int
	GetCurrentWeight() int
	SetCurrentWeight(int)
}

// SelectByWeightedRoundRobin 使用平滑加权轮询算法选择项。
// 该算法确保根据权重公平分配，同时保持平滑的选择顺序。
//
// 步骤：
// 1. 将每个项的权重加到其当前权重
// 2. 选择当前权重最高的项
// 3. 从选中项的当前权重中减去总权重
//
// 这是 Nginx 平滑加权轮询算法。
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
