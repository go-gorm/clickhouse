package clickhouse_test

import (
	"regexp"
	"testing"
	"time"

	"gorm.io/driver/clickhouse"
	"gorm.io/gorm"
	"gorm.io/gorm/utils/tests"
)

func TestUpdateLocalTable(t *testing.T) {
	updateLocalTable := clickhouse.UpdateLocalTable{Suffix: "_local"}
	for k, v := range map[string]string{
		"alter table  hello_world.hello_world2  update id=1":                "alter table  hello_world.hello_world2_local  update id=1",
		"Alter table  `hello_world`.hello_world2  update id=1":              "Alter table  `hello_world`.hello_world2_local  update id=1",
		"ALTER TABLE  hello_world.`hello_world2`  update id=1":              "ALTER TABLE  hello_world.`hello_world2_local`  update id=1",
		"alter TABLE  `hello_world`.`hello_world2`  update id=1":            "alter TABLE  `hello_world`.`hello_world2_local`  update id=1",
		"ALTER TABLE `users` UPDATE `name`=?,`updated_at`=? WHERE `id` = ?": "ALTER TABLE `users_local` UPDATE `name`=?,`updated_at`=? WHERE `id` = ?",
	} {
		if updateLocalTable.ModifySQL(k) != v {
			t.Errorf("failed to update sql, expect: %v, got %v", v, updateLocalTable.ModifySQL(k))
		}
	}

	updateLocalTable = clickhouse.UpdateLocalTable{Prefix: "local_"}
	for k, v := range map[string]string{
		"alter table  hello_world.hello_world2  update id=1":     "alter table  hello_world.local_hello_world2  update id=1",
		"alter table  `hello_world`.hello_world2  update id=1":   "alter table  `hello_world`.local_hello_world2  update id=1",
		"alter table  hello_world.`hello_world2`  update id=1":   "alter table  hello_world.`local_hello_world2`  update id=1",
		"alter table  `hello_world`.`hello_world2`  update id=1": "alter table  `hello_world`.`local_hello_world2`  update id=1",
	} {
		if updateLocalTable.ModifySQL(k) != v {
			t.Errorf("failed to update sql, expect: %v, got %v", v, updateLocalTable.ModifySQL(k))
		}
	}

	updateLocalTable = clickhouse.UpdateLocalTable{Table: "local_table"}
	for k, v := range map[string]string{
		"alter table  hello_world.hello_world2  update id=1":     "alter table  hello_world.local_table  update id=1",
		"ALTER table  `hello_world`.hello_world2  update id=1":   "ALTER table  `hello_world`.local_table  update id=1",
		"alter table  hello_world.`hello_world2`  update id=1":   "alter table  hello_world.`local_table`  update id=1",
		"ALTER TABLE  `hello_world`.`hello_world2`  update id=1": "ALTER TABLE  `hello_world`.`local_table`  update id=1",
	} {
		if updateLocalTable.ModifySQL(k) != v {
			t.Errorf("failed to update sql, expect: %v, got %v", v, updateLocalTable.ModifySQL(k))
		}
	}
}

func TestUpdate(t *testing.T) {
	var user = User{ID: 3, Name: "update", FirstName: "zhang", LastName: "jinzhu", Age: 18, Active: true, Salary: 8.8888}

	if err := DB.Create(&user).Error; err != nil {
		t.Fatalf("failed to create user, got error %v", err)
	}

	var result User
	if err := DB.Find(&result, user.ID).Error; err != nil {
		t.Fatalf("failed to query user, got error %v", err)
	}

	tests.AssertEqual(t, result, user)

	if err := DB.Model(&result).Update("name", "update-1").Error; err != nil {
		t.Fatalf("failed to update user, got error %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	var result2 User
	if err := DB.First(&result2, user.ID).Error; err != nil {
		t.Fatalf("failed to query user, got error %v", err)
	}

	user.Name = "update-1"
	tests.AssertEqual(t, result2, user)

	sql := DB.ToSQL(func(tx *gorm.DB) *gorm.DB {
		return tx.Clauses(clickhouse.UpdateLocalTable{Suffix: "_local"}).Model(&result).Update("name", "update-1")
	})

	if !regexp.MustCompile("`users_local`").MatchString(sql) {
		t.Errorf("Table with namer, got %v", sql)
	}
}

func TestUpdateWithMap(t *testing.T) {
	var user = User{ID: 33, Name: "update2", FirstName: "zhang", LastName: "jinzhu", Age: 18, Active: true, Salary: 8.8888}

	if err := DB.Create(&user).Error; err != nil {
		t.Fatalf("failed to create user, got error %v", err)
	}

	var result User
	if err := DB.Find(&result, user.ID).Error; err != nil {
		t.Fatalf("failed to query user, got error %v", err)
	}

	tests.AssertEqual(t, result, user)

	if err := DB.Table("users").Where("id = ?", user.ID).Update("name", "update-2").Error; err != nil {
		t.Fatalf("failed to update user, got error %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	var result2 User
	if err := DB.First(&result2, user.ID).Error; err != nil {
		t.Fatalf("failed to query user, got error %v", err)
	}

	user.Name = "update-2"
	tests.AssertEqual(t, result2, user)

	if err := DB.Table("users").Where("id = ?", user.ID).Updates(map[string]interface{}{"name": "update-3"}).Error; err != nil {
		t.Fatalf("failed to update user, got error %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	var result3 User
	if err := DB.First(&result3, user.ID).Error; err != nil {
		t.Fatalf("failed to query user, got error %v", err)
	}

	user.Name = "update-3"
	tests.AssertEqual(t, result3, user)
}
