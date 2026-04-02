<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        Schema::table('ssl_certificates', function (Blueprint $table) {
            $table->dropForeign(['domain_id']);
            $table->foreignId('domain_id')->nullable()->change();
            $table->foreign('domain_id')->references('id')->on('domains')->nullOnDelete();
        });
    }

    public function down(): void
    {
        Schema::table('ssl_certificates', function (Blueprint $table) {
            $table->dropForeign(['domain_id']);
            $table->foreignId('domain_id')->nullable(false)->change();
            $table->foreign('domain_id')->references('id')->on('domains')->cascadeOnDelete();
        });
    }
};
