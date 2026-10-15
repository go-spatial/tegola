-- Disposable CI server only. Feature admission inspects the physical InnoDB
-- table identity through INFORMATION_SCHEMA.INNODB_TABLES, which requires
-- PROCESS. The image already grants this fixture user access to test.*.
GRANT PROCESS ON *.* TO 'u'@'%';
