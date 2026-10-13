resource "harness_platform_db_schema" "default_type_test" {
  identifier = "identifier"
  org_id     = "org_id"
  project_id = "project_id"
  name       = "name"
  service    = "service1"
  tags       = ["foo:bar", "bar:foo"]
  schema_source {
    connector    = "gitConnector"
    repo         = "TestRepo"
    location     = "db/example-changelog.yaml"
    archive_path = "path/to/archive.zip"
  }
}

resource "harness_platform_db_schema" "liquibase_repository_test" {
  identifier     = "identifier"
  org_id         = "org_id"
  project_id     = "project_id"
  name           = "name"
  service        = "service1"
  type           = "Repository"
  migration_type = "Liquibase"
  tags           = ["foo:bar", "bar:foo"]
  schema_source {
    connector    = "gitConnector"
    repo         = "TestRepo"
    location     = "db/example-changelog.yaml"
    archive_path = "path/to/archive.zip"
  }
}


resource "harness_platform_db_schema" "liquibase_script_test" {
  identifier     = "identifier"
  org_id         = "org_id"
  project_id     = "project_id"
  name           = "name"
  migration_type = "Liquibase"
  service        = "service1"
  type           = "Script"
  tags           = ["foo:bar", "bar:foo"]
  changelog_script {
    image    = "plugins/image"
    command  = "echo \\\"hello dbops\\\""
    shell    = "sh/bash"
    location = "db/example-changelog.yaml"
  }
}

resource "harness_platform_db_schema" "flyway_repository_test" {
  identifier     = "identifier"
  org_id         = "org_id"
  project_id     = "project_id"
  name           = "name"
  service        = "service1"
  type           = "Repository"
  migration_type = "Flyway"
  tags           = ["foo:bar", "bar:foo"]
  schema_source {
    connector    = "gitConnector"
    repo         = "TestRepo"
    location     = "db/flyway/migrations"
    toml         = "db/flyway.toml"
    archive_path = "path/to/archive.zip"
  }
}


resource "harness_platform_db_schema" "flyway_script_test" {
  identifier     = "identifier"
  org_id         = "org_id"
  project_id     = "project_id"
  name           = "name"
  migration_type = "Flyway"
  service        = "service1"
  type           = "Script"
  tags           = ["foo:bar", "bar:foo"]
  changelog_script {
    image    = "plugins/image"
    command  = "echo \\\"hello dbops\\\""
    shell    = "sh/bash"
    location = "db/flyway/migrations"
    toml     = "db/flyway.toml"
  }
}

resource "harness_platform_db_schema" "percona_enabled_test" {
  identifier     = "identifier"
  org_id         = "org_id"
  project_id     = "project_id"
  name           = "name"
  service        = "service1"
  type           = "Repository"
  migration_type = "Liquibase"
  use_percona    = true
  tags           = ["foo:bar", "bar:foo"]
  schema_source {
    connector    = "gitConnector"
    repo         = "TestRepo"
    location     = "db/example-changelog.yaml"
    archive_path = "path/to/archive.zip"
  }
}

resource "harness_platform_db_schema" "harness_code_repo" {
  identifier = "identifier"
  org_id     = "org_id"
  project_id = "project_id"
  name       = "name"
  service    = "service1"
  type       = "Repository"
  tags       = ["foo:bar", "bar:foo"]
  schema_source {
    repo     = "my-harness-code-repo"
    location = "db/example-changelog.yaml"
  }
}
