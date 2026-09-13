package main

import (
	"encoding/binary"
	"io"
	"math"
)

const bihMaxDepth = 64

type bihPrimitive struct {
	Low, High vector3
	Index     uint32
}

type bihBuilder struct {
	indices []int
	bounds  []bihPrimitive
	tree    []uint32
	low     vector3
	high    vector3
}

func buildBIH(primitives []bihPrimitive) (vector3, vector3, []uint32, []uint32) {
	builder := bihBuilder{bounds: primitives, tree: []uint32{3 << 30, 0, 0}}
	if len(primitives) == 0 {
		return builder.low, builder.high, builder.tree, nil
	}
	builder.indices = make([]int, len(primitives))
	builder.low, builder.high = primitives[0].Low, primitives[0].High
	for index, primitive := range primitives {
		builder.indices[index] = index
		builder.low = minVector(builder.low, primitive.Low)
		builder.high = maxVector(builder.high, primitive.High)
	}
	builder.subdivide(0, len(primitives)-1, builder.low, builder.high, builder.low, builder.high, 0, 1)
	objects := make([]uint32, len(builder.indices))
	for index, primitiveIndex := range builder.indices {
		objects[index] = builder.bounds[primitiveIndex].Index
	}
	return builder.low, builder.high, builder.tree, objects
}

func (b *bihBuilder) subdivide(left, right int, gridLow, gridHigh, nodeLow, nodeHigh vector3, nodeIndex, depth int) {
	if right-left+1 <= 3 || depth >= bihMaxDepth {
		b.createLeaf(nodeIndex, left, right)
		return
	}
	axis := -1
	prevAxis := -1
	rightOrig := right
	clipL, clipR := float32(math.NaN()), float32(math.NaN())
	prevClip := float32(math.NaN())
	split := float32(math.NaN())
	prevSplit := float32(math.NaN())
	wasLeft := true
	for {
		prevAxis, prevSplit = axis, split
		delta := vector3{gridHigh.X - gridLow.X, gridHigh.Y - gridLow.Y, gridHigh.Z - gridLow.Z}
		axis = primaryAxis(delta)
		split = component(gridLow, axis)*0.5 + component(gridHigh, axis)*0.5
		clipL, clipR = float32(math.Inf(-1)), float32(math.Inf(1))
		rightOrig = right
		nodeL, nodeR := float32(math.Inf(1)), float32(math.Inf(-1))
		for index := left; index <= right; {
			primitive := b.bounds[b.indices[index]]
			minBound, maxBound := component(primitive.Low, axis), component(primitive.High, axis)
			center := (minBound + maxBound) * 0.5
			if center <= split {
				index++
				if clipL < maxBound {
					clipL = maxBound
				}
			} else {
				b.indices[index], b.indices[right] = b.indices[right], b.indices[index]
				right--
				if clipR > minBound {
					clipR = minBound
				}
			}
			if nodeL > minBound {
				nodeL = minBound
			}
			if nodeR < maxBound {
				nodeR = maxBound
			}
		}
		if nodeL > component(nodeLow, axis) && nodeR < component(nodeHigh, axis) {
			nodeBoxWidth := component(nodeHigh, axis) - component(nodeLow, axis)
			newWidth := nodeR - nodeL
			if 1.3*newWidth < nodeBoxWidth {
				nextIndex := len(b.tree)
				b.tree = append(b.tree, 0, 0, 0)
				b.tree[nodeIndex] = uint32(axis)<<30 | 1<<29 | uint32(nextIndex)
				b.tree[nodeIndex+1] = math.Float32bits(nodeL)
				b.tree[nodeIndex+2] = math.Float32bits(nodeR)
				updatedLow, updatedHigh := nodeLow, nodeHigh
				setComponent(&updatedLow, axis, nodeL)
				setComponent(&updatedHigh, axis, nodeR)
				b.subdivide(left, rightOrig, gridLow, gridHigh, updatedLow, updatedHigh, nextIndex, depth+1)
				return
			}
		}
		if right == rightOrig {
			if prevAxis == axis && fuzzyEqual(prevSplit, split) {
				b.createLeaf(nodeIndex, left, right)
				return
			}
			if clipL <= split {
				gridHigh = setComponentValue(gridHigh, axis, split)
				prevClip, wasLeft = clipL, true
				continue
			}
			gridHigh = setComponentValue(gridHigh, axis, split)
			prevClip = float32(math.NaN())
		} else if left > right {
			right = rightOrig
			if prevAxis == axis && fuzzyEqual(prevSplit, split) {
				b.createLeaf(nodeIndex, left, right)
				return
			}
			if clipR >= split {
				gridLow = setComponentValue(gridLow, axis, split)
				prevClip, wasLeft = clipR, false
				continue
			}
			gridLow = setComponentValue(gridLow, axis, split)
			prevClip = float32(math.NaN())
		} else {
			if prevAxis != -1 && !math.IsNaN(float64(prevClip)) {
				nextIndex := len(b.tree)
				b.tree = append(b.tree, 0, 0, 0)
				if wasLeft {
					b.tree[nodeIndex] = uint32(prevAxis)<<30 | uint32(nextIndex)
					b.tree[nodeIndex+1] = math.Float32bits(prevClip)
					b.tree[nodeIndex+2] = math.Float32bits(float32(math.Inf(1)))
				} else {
					b.tree[nodeIndex] = uint32(prevAxis)<<30 | uint32(nextIndex-3)
					b.tree[nodeIndex+1] = math.Float32bits(float32(math.Inf(-1)))
					b.tree[nodeIndex+2] = math.Float32bits(prevClip)
				}
				nodeIndex = nextIndex
			}
			break
		}
	}
	nextIndex := len(b.tree)
	nl := right - left + 1
	nr := rightOrig - (right + 1) + 1
	if nl > 0 {
		b.tree = append(b.tree, 0, 0, 0)
	} else {
		nextIndex -= 3
	}
	if nr > 0 {
		b.tree = append(b.tree, 0, 0, 0)
	}
	b.tree[nodeIndex] = uint32(axis)<<30 | uint32(nextIndex)
	b.tree[nodeIndex+1] = math.Float32bits(clipL)
	b.tree[nodeIndex+2] = math.Float32bits(clipR)
	gridLowL, gridHighL := gridLow, gridHigh
	gridLowR, gridHighR := gridLow, gridHigh
	nodeLowL, nodeHighL := nodeLow, nodeHigh
	nodeLowR, nodeHighR := nodeLow, nodeHigh
	gridHighL = setComponentValue(gridHighL, axis, split)
	gridLowR = setComponentValue(gridLowR, axis, split)
	nodeHighL = setComponentValue(nodeHighL, axis, clipL)
	nodeLowR = setComponentValue(nodeLowR, axis, clipR)
	if nl > 0 {
		b.subdivide(left, right, gridLowL, gridHighL, nodeLowL, nodeHighL, nextIndex, depth+1)
	}
	if nr > 0 {
		b.subdivide(right+1, rightOrig, gridLowR, gridHighR, nodeLowR, nodeHighR, nextIndex+3, depth+1)
	}
}

func (b *bihBuilder) createLeaf(nodeIndex, left, right int) {
	b.tree[nodeIndex] = 3<<30 | uint32(left)
	b.tree[nodeIndex+1] = uint32(right - left + 1)
	b.tree[nodeIndex+2] = 0
}

func writeBIH(writer io.Writer, primitives []bihPrimitive) error {
	low, high, tree, objects := buildBIH(primitives)
	for _, value := range []any{low, high, uint32(len(tree))} {
		if err := binary.Write(writer, binary.LittleEndian, value); err != nil {
			return err
		}
	}
	if err := binary.Write(writer, binary.LittleEndian, tree); err != nil {
		return err
	}
	if err := binary.Write(writer, binary.LittleEndian, uint32(len(objects))); err != nil {
		return err
	}
	return binary.Write(writer, binary.LittleEndian, objects)
}

func primaryAxis(value vector3) int {
	axis := 0
	if value.Y > value.X {
		axis = 1
	}
	if component(value, 2) > component(value, axis) {
		axis = 2
	}
	return axis
}

func component(value vector3, axis int) float32 {
	if axis == 0 {
		return value.X
	}
	if axis == 1 {
		return value.Y
	}
	return value.Z
}

func setComponent(value *vector3, axis int, componentValue float32) {
	if axis == 0 {
		value.X = componentValue
	} else if axis == 1 {
		value.Y = componentValue
	} else {
		value.Z = componentValue
	}
}

func setComponentValue(value vector3, axis int, componentValue float32) vector3 {
	setComponent(&value, axis, componentValue)
	return value
}

func fuzzyEqual(a, b float32) bool {
	return a == b || math.Abs(float64(a-b)) <= 1e-6
}
