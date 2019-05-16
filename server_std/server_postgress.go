package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	"os"
	"github.com/jinzhu/gorm"
	"time"
	"strconv"
)

type Log struct	{
	Id		int64		`sql:"id"`
	UserId		int64		`sql:"user_id"`
	CreatedAt	time.Time	`sql:"created_at"`
	EventType	int		`sql:"event_type"`
}

type User struct {
	Id            int64      `gorm:"primary_key" json:"id"`
	CardKey       int64      `sql:"card_key" json:"card_key"`
	FirstName     string     `sql:"first_name" json:"first_name"`
	LastName      string     `sql:"last_name" json:"last_name"`
	Status        int64      `gorm:"default:1" sql:"status" json:"status"`
	LastCheckedIn *time.Time `sql:"last_checked_in" json:"last_checked_in,omitempty"`
	Active        *bool      `gorm:"default:true" sql:"active" json:"active"`
}



var db  *gorm.DB


func main() {

	host := os.Getenv("HOST")
	user := os.Getenv("USER")
	password := os.Getenv("PASSWORD")
	dbName := os.Getenv("DB_NAME")

	connStr := fmt.Sprintf("host=%v user=%v dbname=%v sslmode=require password=%v", host, user, dbName, password)
	ddb, err := gorm.Open("postgres", connStr)
	db = ddb
	if err != nil {
		panic("Failed to connect database")
	}
	ddb.LogMode(true)

	defer ddb.Close()

	router := mux.NewRouter()

        router.HandleFunc("/{any:.*}", Options).Methods("OPTIONS")
	
	router.HandleFunc("/std/user", GetResources).Methods("GET")
	router.HandleFunc("/std/user/{card_key}", GetResource).Methods("GET")
	router.HandleFunc("/std/user", CreateResource).Methods("POST")
	router.HandleFunc("/std/user/update/{id}", UpdateResource).Methods("PUT")
	
	router.HandleFunc("/std/user/deactivate/{id}", DeactiveUser).Methods("PUT")
	router.HandleFunc("/std/user/activate/{id}", ActiveUser).Methods("PUT")
	
	router.HandleFunc("/std/user/blocked/{id}",BlockedUser).Methods("PUT")
	router.HandleFunc("/std/user/unblocked/{id}",UnblockedUser).Methods("PUT")
	
	router.HandleFunc("/std/auth",AuthUser).Methods("POST")
	router.HandleFunc("/std/exit",UserExit).Methods("POST")
	
	router.HandleFunc("/std/logs", GetLogs).Methods("GET")
	router.HandleFunc("/std/logs/{user_id}", GetLog).Methods("GET")

	http.ListenAndServe(":" + os.Getenv("PORT"), router)

}

func Options(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Access-Control-Allow-Origin", "*")
	w.Header().Add("Access-Control-Allow-Methods", "POST, GET, PUT, OPTIONS, PATCH, DELETE")
}

func WriteResult(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Add("Access-Control-Allow-Origin", "*")
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(code)

	if data != nil {
		json.NewEncoder(w).Encode(data)
	}
}

func Event(UserId int64, EventType int) {
	log := Log{UserId: UserId, CreatedAt: time.Now(), EventType: EventType}
	if err := db.Create(&log).Error; err != nil {
		fmt.Println(err.Error())
	}
}

func GetResources(w http.ResponseWriter, r *http.Request) {
	var users []User
	if err := db.Where("active = ?", true).Order("id ASC").Find(&users).Error; err != nil {
		WriteResult(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteResult(w, http.StatusOK, users)

}

func GetLog(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	var logs []Log
	i, err := strconv.ParseInt(params["user_id"], 10, 64)

	if err != nil {
		WriteResult(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := db.Where("user_id = ?", i).Find(&logs).Error; err != nil {
		WriteResult(w, http.StatusInternalServerError, err.Error())
		return
	}
	WriteResult(w, http.StatusOK, logs)

}

func GetLogs(w http.ResponseWriter, r *http.Request) {
	var logs []Log
	if err := db.Order("id DESC").Find(&logs).Error; err != nil {
		WriteResult(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteResult(w, http.StatusOK, logs)

}

func GetResource(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	var resource User
	i, err := strconv.ParseInt(params["card_key"], 10, 64)

	if err != nil {
		WriteResult(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := db.Where("card_key = ?", i).First(&resource).Error; err != nil {
		WriteResult(w, http.StatusNotFound, http.StatusNotFound)
		return
	}

	if err := db.Where("status = ?", 0).Where("active", true).First(&resource).Error; err != nil {
		WriteResult(w, http.StatusOK, resource)
		return
	}

	WriteResult(w, http.StatusForbidden, http.StatusForbidden)

}

func CreateResource(w http.ResponseWriter, r *http.Request) {
	var resource User
	if err := json.NewDecoder(r.Body).Decode(&resource); err != nil {
		WriteResult(w, http.StatusBadRequest, err.Error())
		return
	}
	defer r.Body.Close()

	if err := db.Create(&resource).Error; err != nil {
		WriteResult(w, http.StatusBadRequest, err.Error())
		return
	}

	go Event(resource.Id, 6)

	WriteResult(w, http.StatusOK, resource)
}

func UpdateResource(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		WriteResult(w, http.StatusNotFound, err.Error())
		return
	}

	var resource User
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&resource); err != nil {
		WriteResult(w, http.StatusBadRequest, err.Error())
		return
	}
	defer r.Body.Close()

	resource.Id = id
	if err := db.Save(&resource).Error; err != nil {
		WriteResult(w, http.StatusInternalServerError, err)
		return
	}
	go Event(id, 7)

	WriteResult(w, http.StatusOK, resource)
}

func BlockedUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		WriteResult(w, http.StatusNotFound, err.Error())
		return
	}
	var user User

	user.Id = id

	if err := db.First(&user).Error; err != nil {
		WriteResult(w, http.StatusBadRequest, err.Error())
		return
	}

	user.Status = 0

	if err := db.Save(&user).Error; err != nil {
		WriteResult(w, http.StatusInternalServerError, err)
		return
	}

	go Event(id, 4)

	WriteResult(w, http.StatusOK, user)

}

func UnblockedUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		WriteResult(w, http.StatusNotFound, err.Error())
		return
	}
	var user User

	user.Id = id

	if err := db.First(&user).Error; err != nil {
		WriteResult(w, http.StatusBadRequest, err.Error())
		return
	}

	user.Status = 1

	if err := db.Save(&user).Error; err != nil {
		WriteResult(w, http.StatusInternalServerError, err)
		return
	}

	go Event(id, 5)

	WriteResult(w, http.StatusOK, user)
}

func AuthUser(w http.ResponseWriter, r *http.Request) {

	var resource User
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&resource);
	if err != nil {
		WriteResult(w, http.StatusBadRequest, err.Error())
		return
	}
	defer r.Body.Close()

	if err := db.Where("card_key = ?", resource.CardKey).First(&resource).Error; err != nil {

		go Event(0, 3)

		WriteResult(w, http.StatusNotFound, nil)
		return
	}

	if resource.Status == 1 {

		if resource.Active != nil && *resource.Active == true {
			moment := time.Now()
			resource.LastCheckedIn = &moment
			if err := db.Save(&resource).Error; err != nil {
				WriteResult(w, http.StatusInternalServerError, err)
				return
			}

			go Event(resource.Id, 1)

			WriteResult(w, http.StatusOK, nil)
		}
	} else {

		go Event(resource.Id, 2)

		WriteResult(w, http.StatusForbidden, nil)
	}

}

func UserExit(w http.ResponseWriter, r *http.Request) {

	go Event(0, 9)

	WriteResult(w, http.StatusOK, nil)
}

func DeactiveUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		WriteResult(w, http.StatusNotFound, err.Error())
		return
	}
	var user User

	user.Id = id

	if err := db.First(&user).Error; err != nil {
		WriteResult(w, http.StatusBadRequest, err.Error())
		return
	}
	active := false
	user.Active = &active

	if err := db.Save(&user).Error; err != nil {
		WriteResult(w, http.StatusInternalServerError, err)
		return
	}

	go Event(id, 8)

	WriteResult(w, http.StatusOK, user)

}

func ActiveUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		WriteResult(w, http.StatusNotFound, err.Error())
		return
	}
	var user User

	user.Id = id

	if err := db.First(&user).Error; err != nil {
		WriteResult(w, http.StatusBadRequest, err.Error())
		return
	}
	active := true
	user.Active = &active

	if err := db.Save(&user).Error; err != nil {
		WriteResult(w, http.StatusInternalServerError, err)
		return
	}

	go Event(id, 10)

	WriteResult(w, http.StatusOK, user)

}
