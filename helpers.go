package notifier

func subset(first, second []string) bool {
	set := make(map[string]bool)
	for _, value := range second {
		set[value] = true
	}

	for _, value := range first {
		if !set[value] {
			return false
		}
	}

	return true
}
