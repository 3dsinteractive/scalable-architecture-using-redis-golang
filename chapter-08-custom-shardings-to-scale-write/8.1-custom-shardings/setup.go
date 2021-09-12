package main

func setup(cfg IConfig) error {

	// Clear all caches
	cfgs := []ICacherConfig{
		cfg.CacherConfig1(),
		cfg.CacherConfig2(),
		cfg.CacherConfig3(),
		cfg.CacherConfig4(),
		cfg.CacherConfig5(),
	}
	for _, cacheCfg := range cfgs {
		cacher := NewCacher(cacheCfg)
		allKeys, err := cacher.Keys("*")
		if err != nil {
			return err
		}

		err = cacher.Del(allKeys...)
		if err != nil {
			return err
		}
	}

	return nil
}
