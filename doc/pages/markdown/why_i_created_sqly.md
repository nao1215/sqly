I developed sqly to easily check large CSV files.

In a project at my company in which I was employed from 2022 to 2025, I manage the master data of an app using CSV files. These CSV files have the following characteristics and constraints:

- Large size (over 20,000 rows × 300 columns, or 100,000 rows)
- Read CSV files with Golang and insert records into multiple DB tables
- CSV and DB tables do not correspond one-to-one (data from one CSV is inserted into multiple tables)
- The people editing the CSV files are not engineers, and there are multiple editors
- The CSV files are updated several times a month

Considering the above characteristics and constraints, the difficulties of using CSV files are as follows:

- It takes time to launch Excel/Numbers/Google Sheets (they often crash)
- Type mismatch errors occur when importing to the DB (due to mistakes in CSV columns), and it is costly to find the problematic parts

For example, if a string is written in a column where a number should be, a decode error occurs. Unfortunately, the error message is "A decode error occurred! (I won't tell you which column is bad!)", so you have to manually find the problematic column.

Using Google Sheets to find the problematic column among over 300 columns is very stressful. It is not an engineer's job. Therefore, I developed sqly to search with SQL and make things easier.
