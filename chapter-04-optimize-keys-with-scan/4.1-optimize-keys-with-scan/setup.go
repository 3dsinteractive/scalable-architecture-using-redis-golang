package main

import "fmt"

func setup(cfg IConfig) error {

	// Clear all caches
	cacher := NewCacher(cfg.CacherConfig())
	allKeys, err := cacher.Keys("*")
	if err != nil {
		return err
	}

	err = cacher.Del(allKeys...)
	if err != nil {
		return err
	}

	// Add 300k keys into redis (in 3 batch)
	batchSize := 100000
	batchRound := 3
	for i := 0; i < batchRound; i++ {
		kvs := make(map[string]interface{})
		for j := 0; j < batchSize; j++ {
			id := i*batchSize + j
			key := fmt.Sprintf("key::%d", id)
			kvs[key] = id
		}
		err = cacher.MSet(kvs)
		if err != nil {
			return err
		}
	}

	return nil
}
