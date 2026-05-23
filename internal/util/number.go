package util

import (
	"fmt"
	"strconv"
)

func IntRange(start int, end int) []int {
	length := end - start + 1
	if length < 0 {
		return nil
	}
	result := make([]int, length)
	for i := start; i <= end; i++ {
		result[i-start] = i
	}
	return result
}

func SafeParseInt[T ~int | int64](str string, fallbackValue T) T {
	val, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return fallbackValue
	}
	return T(val)
}

func SafeParseFloat(str string, fallbackValue float64) float64 {
	val, err := strconv.ParseFloat(str, 64)
	if err != nil {
		return fallbackValue
	}
	return val
}

func ZeroPadInt(n int, length int) string {
	return fmt.Sprintf("%0"+strconv.Itoa(length)+"d", n)
}

func IntToString[T ~int | int8 | int16 | int32 | int64](i T) string {
	return strconv.FormatInt(int64(i), 10)
}

// Percentile returns the p-th percentile from a sorted slice using linear interpolation.
func Percentile(sorted []float64, p float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n == 1 {
		return sorted[0]
	}
	rank := (p / 100) * float64(n-1)
	lower := int(rank)
	upper := lower + 1
	if upper >= n {
		return sorted[n-1]
	}
	frac := rank - float64(lower)
	return sorted[lower] + frac*(sorted[upper]-sorted[lower])
}
