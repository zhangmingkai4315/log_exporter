package collector

// merger deep merge a map[string]map[string]int object
// https://play.golang.org/p/QYDbdJgcSmP
func merger(managers []map[string]map[string]uint64) map[string]map[string]uint64 {
	temp := map[string]map[string]uint64(managers[0])
	for i, r := range managers {
		if i == 0 {
			continue
		}
		for k, v := range r {
			if tv, ok := temp[k]; ok {
				for vk, vv := range v {
					if tvk, ok := tv[vk]; ok {
						tv[vk] = tvk + vv
					} else {
						tv[vk] = vv
					}
				}
			} else {
				temp[k] = v
			}
		}
	}
	return temp
}
