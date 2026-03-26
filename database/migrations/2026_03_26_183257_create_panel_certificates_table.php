<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        Schema::create('panel_certificates', function (Blueprint $table) {
            $table->id();
            $table->string('hostname');
            $table->string('status')->default('pending');
            $table->string('issuer')->nullable();
            $table->timestamp('issued_at')->nullable();
            $table->timestamp('expires_at')->nullable();
            $table->timestamp('last_renewal_at')->nullable();
            $table->string('last_renewal_result')->nullable();
            $table->text('last_error')->nullable();
            $table->string('serial_number')->nullable();
            $table->timestamps();
        });
    }

    public function down(): void
    {
        Schema::dropIfExists('panel_certificates');
    }
};
