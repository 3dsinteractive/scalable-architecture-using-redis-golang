package main

import "fmt"

func setup(cfg IConfig) error {

	pst := NewPersister(cfg.PersisterConfig())
	exists, err := pst.TableExists(&Member{})
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	// Create table for test
	err = pst.Exec(RemoveTabAndNewLine(
		`CREATE TABLE members (
			id VARCHAR(50) NOT NULL,
			username VARCHAR(500) NOT NULL,
			register_order INT NOT NULL,
			is_active INT NOT NULL,
			CONSTRAINT PK_members PRIMARY KEY (id)
		);`))
	if err != nil {
		return err
	}

	numberOfMembers := 100000
	members := make([]*Member, numberOfMembers)
	for i := 0; i < numberOfMembers; i++ {
		id := i
		// Half of member is active, another half is inactive
		isActive := 0
		if i%2 == 0 {
			isActive = 1
		}

		member := &Member{
			ID:            fmt.Sprintf("id_%d", id),
			Username:      fmt.Sprintf("user_%d", id),
			RegisterOrder: i,
			IsActive:      isActive,
		}
		members[i] = member
	}

	err = pst.CreateInBatch(members, 1000)
	if err != nil {
		return err
	}

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
