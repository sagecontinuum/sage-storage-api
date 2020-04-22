

CREATE TABLE IF NOT EXISTS SageStorage.Buckets (
    id                  BINARY(16) NOT NULL PRIMARY KEY,
    name                VARCHAR(64),
    type                VARCHAR(64),
    time_created        TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    time_last_updated   TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    owner               VARCHAR(64)
);


CREATE TABLE IF NOT EXISTS SageStorage.BucketPermissions (
    id                  BINARY(16) NOT NULL,
    granteeType         ENUM('USER', 'GROUP'),
    grantee             VARCHAR(64), 
    permission          ENUM('READ', 'WRITE', 'READ_ACP', 'WRITE_ACP', 'FULL_CONTROL'),
    PRIMARY KEY (id, granteeType, grantee, permission)
);
# permissions similar to https://docs.aws.amazon.com/AmazonS3/latest/dev/acl-overview.html


