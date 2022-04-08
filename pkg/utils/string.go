package utils

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

func ContainsString(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

func ParseFloat(str string, defaultValue float64) (float64, error) {
	if len(str) == 0 {
		return defaultValue, nil
	}
	return strconv.ParseFloat(str, 64)
}

// parsePercentage parse the percent string value
func ParsePercentage(input string) (float64, error) {
	if len(input) == 0 {
		return 0, nil
	}
	value, err := strconv.ParseFloat(strings.TrimRight(input, "%"), 64)
	if err != nil {
		return 0, err
	}
	return value / 100, nil
}

// See: http://man7.org/linux/man-pages/man7/cpuset.7.html#FORMATS
func ParseRangeToSlice(input string) ([]int, error) {
	rMap := make(map[int]struct{})

	//0-2,7,12-14 ==> [0-2, 7, 12-14]
	ranges := strings.Split(input, ",")

	for _, r := range ranges {
		boundaries := strings.SplitN(r, "-", 2)
		if len(boundaries) == 1 {
			elem, err := strconv.Atoi(boundaries[0])
			if err != nil {
				return []int{}, err
			}
			rMap[elem] = struct{}{}

			//0-2 ==> [0, 1, 2]
		} else if len(boundaries) == 2 {
			start, err := strconv.Atoi(boundaries[0])
			if err != nil {
				return []int{}, err
			}
			end, err := strconv.Atoi(boundaries[1])
			if err != nil {
				return []int{}, err
			}
			if start > end {
				return []int{}, fmt.Errorf("invalid range %q (%d >= %d)", r, start, end)
			}

			for e := start; e <= end; e++ {
				rMap[e] = struct{}{}
			}
		}
	}

	result := make([]int, 0, len(rMap))

	for elem := range rMap {
		result = append(result, elem)
	}
	sort.Ints(result)

	return result, nil
}