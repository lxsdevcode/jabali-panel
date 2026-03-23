<?php

declare(strict_types=1);

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        Schema::table('backups', function (Blueprint $table) {
            $table->boolean('include_ssl')->default(true)->after('include_dns');
        });

        Schema::table('backup_schedules', function (Blueprint $table) {
            $table->boolean('include_ssl')->default(true)->after('include_dns');
        });
    }

    public function down(): void
    {
        Schema::table('backups', function (Blueprint $table) {
            $table->dropColumn('include_ssl');
        });

        Schema::table('backup_schedules', function (Blueprint $table) {
            $table->dropColumn('include_ssl');
        });
    }
};
