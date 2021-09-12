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
			member_level INT NOT NULL,
			is_active INT NOT NULL,
			CONSTRAINT PK_members PRIMARY KEY (id)
		);`))
	if err != nil {
		return err
	}

	err = pst.Exec(RemoveTabAndNewLine(
		`CREATE TABLE member_points (
			username VARCHAR(500) NOT NULL,
			point INT NOT NULL,
			CONSTRAINT PK_member_points PRIMARY KEY (username)
		);`))
	if err != nil {
		return err
	}

	// Seed member to table members
	numberOfMembers := 100000
	members := make([]*Member, numberOfMembers)
	memberPoints := make([]*MemberPoint, numberOfMembers)
	for i := 0; i < numberOfMembers; i++ {
		id := i
		// Half of member is active, another half is inactive
		isActive := 0
		if i%2 == 0 {
			isActive = 1
		}

		userName := fmt.Sprintf("user_%d", id)
		member := &Member{
			ID:            fmt.Sprintf("id_%d", id),
			Username:      userName,
			MemberLevel:   RandomMinMax(1, 5),
			RegisterOrder: i,
			IsActive:      isActive,
		}

		memberPoint := &MemberPoint{
			Username: userName,
			Point:    RandomMinMax(0, 1000),
		}

		members[i] = member
		memberPoints[i] = memberPoint
	}

	err = pst.CreateInBatch(members, 1000)
	if err != nil {
		return err
	}
	err = pst.CreateInBatch(memberPoints, 1000)
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
