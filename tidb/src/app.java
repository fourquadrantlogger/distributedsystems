

import java.sql.Connection;
import java.sql.DriverManager;
import java.sql.SQLException;
import java.util.Date;

import com.mysql.cj.jdbc.PreparedStatement;

public class app {

    public static void main(String[] args) throws ClassNotFoundException, SQLException {
        final String url = "jdbc:mysql://192.168.199.82:4000/teacher"+"user=paidian&password=Paidian2016&useUnicode=true&characterEncoding=UTF8&useSSL=false";

        Connection conn = null;
        Class.forName("com.mysql.cj.jdbc.Driver");//指定连接类型

        conn = DriverManager.getConnection(url);//获取连接
        if (conn!=null) {
            System.out.println("获取连接成功");
            insert(conn);
        }else {
            System.out.println("获取连接失败");
        }

    }
    public static void insert(Connection conn) {
        // 开始时间
        Long begin = new Date().getTime();
        // sql前缀
        String prefix = "INSERT INTO user (id,name,age,sex,description,pic_url,school_name,registtime) VALUES ";
        try {
            // 保存sql后缀
            StringBuffer suffix = new StringBuffer();
            // 设置事务为非自动提交
            conn.setAutoCommit(false);
            // 比起st，pst会更好些
            PreparedStatement  pst = (PreparedStatement) conn.prepareStatement("");//准备执行语句
            // 外层循环，总提交事务次数
            for (int i = 1; i <= 1000; i++) {
                suffix = new StringBuffer();
                // 第j次提交步长
                for (int j = 1; j <= 100000; j++) {
                    // 构建SQL后缀
                    suffix.append("('" + (int)Math.random()+"','"+i*j+"','123'"+ ",'男'"+",'教师'"+",'www.bbk.com'"+",'XX大学'"+",'"+new Date().getTime()/1000+"'" +"),");
                }
                // 构建完整SQL
                String sql = prefix + suffix.substring(0, suffix.length() - 1);
                // 添加执行SQL
                pst.addBatch(sql);
                // 执行操作
                pst.executeBatch();
                // 提交事务
                conn.commit();
                // 清空上一次添加的数据
                suffix = new StringBuffer();
            }
            // 头等连接
            pst.close();
            conn.close();
        } catch (SQLException e) {
            e.printStackTrace();
        }
        // 结束时间
        Long end = new Date().getTime();
        // 耗时
        System.out.println("10000万条数据插入花费时间 : " + (end - begin) / 1000 + " s");
        System.out.println("插入完成");
    }
}