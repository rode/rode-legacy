package controllers

func containsFinalizer(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

func removeFinalizer(slice []string, str string) []string {
	var result []string

	for _, s := range slice {
		if s == str {
			continue
		}

		result = append(result, s)
	}

	return result
}
