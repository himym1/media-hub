package emby

import (
	"sort"
	"strings"
)

func normalizeBrowseSort(sortValue string, adult bool) (key string, desc bool) {
	switch strings.ToLower(strings.TrimSpace(sortValue)) {
	case "year-desc":
		return "year", true
	case "year-asc":
		return "year", false
	case "name-asc", "name":
		return "name", false
	case "added-desc", "added":
		return "added", true
	default:
		if adult {
			return "added", true
		}
		return "name", false
	}
}

func itemSortLess(left, right baseItem, key string) bool {
	switch key {
	case "year":
		if left.ProductionYear != right.ProductionYear {
			return left.ProductionYear < right.ProductionYear
		}
		if left.DateCreated != right.DateCreated {
			return left.DateCreated < right.DateCreated
		}
		return strings.ToLower(left.Name) < strings.ToLower(right.Name)
	case "added":
		if left.DateCreated != right.DateCreated {
			return left.DateCreated < right.DateCreated
		}
		return strings.ToLower(left.Name) < strings.ToLower(right.Name)
	default:
		return strings.ToLower(left.Name) < strings.ToLower(right.Name)
	}
}

func sortBaseItems(items []baseItem, sortValue string, adult bool) {
	key, desc := normalizeBrowseSort(sortValue, adult)
	sort.SliceStable(items, func(i, j int) bool {
		if desc {
			return itemSortLess(items[j], items[i], key)
		}
		return itemSortLess(items[i], items[j], key)
	})
}

func paginateBaseItems(items []baseItem, offset, limit int) []baseItem {
	if offset > len(items) {
		offset = len(items)
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end]
}
