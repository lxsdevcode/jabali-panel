<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        Schema::table('domains', function (Blueprint $table) {
            $table->string('dns_status')->nullable()->after('is_active');
            $table->string('dns_resolved_ip')->nullable()->after('dns_status');
            $table->timestamp('dns_checked_at')->nullable()->after('dns_resolved_ip');
            $table->string('whois_status')->nullable()->after('dns_checked_at');
            $table->date('whois_expiry')->nullable()->after('whois_status');
        });
    }

    public function down(): void
    {
        Schema::table('domains', function (Blueprint $table) {
            $table->dropColumn(['dns_status', 'dns_resolved_ip', 'dns_checked_at', 'whois_status', 'whois_expiry']);
        });
    }
};
