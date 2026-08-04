SET SESSION group_concat_max_len = 1048576;

SELECT GROUP_CONCAT(
         CONCAT(
           'SELECT ',
           QUOTE(table_name),
           ' AS table_name, COUNT(*) AS row_count FROM `',
           REPLACE(table_name, '`', '``'),
           '`'
         )
         ORDER BY table_name
         SEPARATOR ' UNION ALL '
       )
  INTO @count_sql
  FROM information_schema.tables
 WHERE table_schema = DATABASE()
   AND table_type = 'BASE TABLE';

PREPARE count_statement FROM @count_sql;
EXECUTE count_statement;
DEALLOCATE PREPARE count_statement;
