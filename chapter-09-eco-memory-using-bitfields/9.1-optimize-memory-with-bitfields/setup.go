package main

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

	return nil
}
