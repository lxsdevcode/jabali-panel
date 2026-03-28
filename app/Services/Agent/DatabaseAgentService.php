<?php

declare(strict_types=1);

namespace App\Services\Agent;

class DatabaseAgentService
{
    public function __construct(private AgentClient $agent) {}

    // MySQL operations

    public function mysqlListDatabases(string $username): array
    {
        return $this->agent->mysqlListDatabases($username);
    }

    public function mysqlCreateDatabase(string $username, string $database): array
    {
        return $this->agent->mysqlCreateDatabase($username, $database);
    }

    public function mysqlDeleteDatabase(string $username, string $database): array
    {
        return $this->agent->mysqlDeleteDatabase($username, $database);
    }

    public function mysqlListUsers(string $username): array
    {
        return $this->agent->mysqlListUsers($username);
    }

    public function mysqlCreateUser(string $username, string $dbUser, string $password, string $host = 'localhost'): array
    {
        return $this->agent->mysqlCreateUser($username, $dbUser, $password, $host);
    }

    public function mysqlDeleteUser(string $username, string $dbUser, string $host = 'localhost'): array
    {
        return $this->agent->mysqlDeleteUser($username, $dbUser, $host);
    }

    public function mysqlChangePassword(string $username, string $dbUser, string $password, string $host = 'localhost'): array
    {
        return $this->agent->mysqlChangePassword($username, $dbUser, $password, $host);
    }

    public function mysqlGrantPrivileges(string $username, string $dbUser, string $database, array $privileges = ['ALL'], string $host = 'localhost'): array
    {
        return $this->agent->mysqlGrantPrivileges($username, $dbUser, $database, $privileges, $host);
    }

    public function mysqlRevokePrivileges(string $username, string $dbUser, string $database, string $host = 'localhost'): array
    {
        return $this->agent->mysqlRevokePrivileges($username, $dbUser, $database, $host);
    }

    public function mysqlGetPrivileges(string $username, string $dbUser, string $host = 'localhost'): array
    {
        return $this->agent->mysqlGetPrivileges($username, $dbUser, $host);
    }

    public function mysqlExportDatabase(string $username, string $database, string $outputFile, string $compress = 'gz'): array
    {
        return $this->agent->mysqlExportDatabase($username, $database, $outputFile, $compress);
    }

    public function mysqlImportDatabase(string $username, string $database, string $sqlFile): array
    {
        return $this->agent->mysqlImportDatabase($username, $database, $sqlFile);
    }

    public function mysqlCreateMasterUser(string $username): array
    {
        return $this->agent->mysqlCreateMasterUser($username);
    }

    // PostgreSQL operations

    public function postgresListDatabases(string $username): array
    {
        return $this->agent->postgresListDatabases($username);
    }

    public function postgresCreateDatabase(string $username, string $database, string $owner): array
    {
        return $this->agent->postgresCreateDatabase($username, $database, $owner);
    }

    public function postgresDeleteDatabase(string $username, string $database): array
    {
        return $this->agent->postgresDeleteDatabase($username, $database);
    }

    public function postgresListUsers(string $username): array
    {
        return $this->agent->postgresListUsers($username);
    }

    public function postgresCreateUser(string $username, string $dbUser, string $password): array
    {
        return $this->agent->postgresCreateUser($username, $dbUser, $password);
    }

    public function postgresDeleteUser(string $username, string $dbUser): array
    {
        return $this->agent->postgresDeleteUser($username, $dbUser);
    }

    public function postgresChangePassword(string $username, string $dbUser, string $password): array
    {
        return $this->agent->postgresChangePassword($username, $dbUser, $password);
    }

    public function postgresGrantPrivileges(string $username, string $database, string $dbUser): array
    {
        return $this->agent->postgresGrantPrivileges($username, $database, $dbUser);
    }

    // Generic database operations

    /**
     * @param  array<int, string>  $names
     */
    public function getVariables(array $names): array
    {
        return $this->agent->databaseGetVariables($names);
    }

    public function setGlobal(string $name, string $value): array
    {
        return $this->agent->databaseSetGlobal($name, $value);
    }

    public function persistTuning(string $name, string $value): array
    {
        return $this->agent->databasePersistTuning($name, $value);
    }
}
