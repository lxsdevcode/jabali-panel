<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        Schema::table('domains', function (Blueprint $table) {
            $table->text('custom_nginx_directives')->nullable()->after('page_cache_enabled');
            $table->json('custom_nginx_rules')->nullable()->after('custom_nginx_directives');
        });
    }

    public function down(): void
    {
        Schema::table('domains', function (Blueprint $table) {
            $table->dropColumn(['custom_nginx_directives', 'custom_nginx_rules']);
        });
    }
};
