package main



import (

	"database/sql"

	"fmt"

	_ "github.com/go-sql-driver/mysql"

	"strconv"
)



func main() {

	insert()

}

const (
	i_length  =1000
	j_length  =100000
)
var (
	db *sql.DB
)
func getdb()(*sql.DB){
	if(db!=nil){
		return db
	}else {
		db, err := sql.Open("mysql", "paidian:Paidian2016@tcp(192.168.199.82:4000)/teacher?charset=utf8")
		checkErr(err)
		return db
	}
}
//插入demo

func insert() {
	for i:=0;i<i_length;i++{
		sqlstr:=`INSERT INTO user (id,name) VALUES`
		for j:=0;j<j_length;j++{
			sqlstr+="("+strconv.Itoa(i*j_length+j)+",'name"+strconv.Itoa(i*j_length+j)+"'),"
		}
		sqlstr=sqlstr[:len(sqlstr)-1]
		res, err := getdb().Exec(sqlstr)
		fmt.Println(err)
		id, err := res.LastInsertId()
		fmt.Println(err)
		fmt.Println(id)
	}

}



//查询demo

func query() {
	rows, err := getdb().Query("SELECT * FROM user")

	checkErr(err)



	//普通demo

	//for rows.Next() {

	//    var userId int

	//    var userName string

	//    var userAge int

	//    var userSex int



	//    rows.Columns()

	//    err = rows.Scan(&userId, &userName, &userAge, &userSex)

	//    checkErr(err)



	//    fmt.Println(userId)

	//    fmt.Println(userName)

	//    fmt.Println(userAge)

	//    fmt.Println(userSex)

	//}



	//字典类型

	//构造scanArgs、values两个数组，scanArgs的每个值指向values相应值的地址

	columns, _ := rows.Columns()

	scanArgs := make([]interface{}, len(columns))

	values := make([]interface{}, len(columns))

	for
	i := range
		values {

		scanArgs[i] = &values[i]

	}



	for
		rows.Next() {

		//将行数据保存到record字典

		err = rows.Scan(scanArgs...)

		record := make(map[string]string)

		for
		i, col := range
			values {

			if
			col != nil {

				record[columns[i]] = string(col.([]byte))

			}

		}

		fmt.Println(record)

	}

}



//更新数据

func update() {
	stmt, err := getdb().Prepare(`UPDATE user SET user_age=?,user_sex=? WHERE user_id=?`)

	checkErr(err)

	res, err := stmt.Exec(21, 2, 1)

	checkErr(err)

	num, err := res.RowsAffected()

	checkErr(err)

	fmt.Println(num)

}



//删除数据

func
remove() {



	stmt, err := getdb().Prepare(`DELETE FROM user WHERE user_id=?`)

	checkErr(err)

	res, err := stmt.Exec(1)

	checkErr(err)

	num, err := res.RowsAffected()

	checkErr(err)

	fmt.Println(num)

}



func checkErr(err error) {

	if
	err != nil {

		panic(err)

	}

}