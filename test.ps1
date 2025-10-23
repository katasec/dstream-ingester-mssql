
$testConfig = @"
{
  "db_connection_string": "server=localhost,1433;user id=sa;password=Passw0rd123;database=TestDB;encrypt=disable",
  "poll_interval": "5s",
  "max_poll_interval": "30s",
  "lock_config": {
    "type": "none",
    "connection_string": "",
    "container_name": ""
  },
  "tables": [
    "Cars",
    "Persons"
  ]
}
"@
 
echo $testConfig | timeout 10s ./dstream-ingester-mssql