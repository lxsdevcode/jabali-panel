<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        Schema::table('backup_restores', function (Blueprint $table) {
            $table->foreignId('snapshot_id')->nullable()->after('backup_id')
                ->constrained('user_remote_backups')->nullOnDelete();
            $table->boolean('restore_ssl')->default(true)->after('restore_dns');
            $table->boolean('restore_mysql_users')->default(true)->after('restore_ssl');
        });

        // Make backup_id nullable for snapshot-based restores
        Schema::table('backup_restores', function (Blueprint $table) {
            $table->unsignedBigInteger('backup_id')->nullable()->change();
        });
    }

    public function down(): void
    {
        Schema::table('backup_restores', function (Blueprint $table) {
            $table->dropForeign(['snapshot_id']);
            $table->dropColumn(['snapshot_id', 'restore_ssl', 'restore_mysql_users']);
        });
    }
};
