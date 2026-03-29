<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    /**
     * Run the migrations.
     */
    public function up(): void
    {
        Schema::create('domain_bandwidth_usage', function (Blueprint $table) {
            $table->id();
            $table->foreignId('domain_id')->constrained()->cascadeOnDelete();
            $table->date('date');
            $table->unsignedBigInteger('bytes_out')->default(0);
            $table->unsignedBigInteger('requests')->default(0);
            $table->timestamp('created_at')->nullable();

            $table->unique(['domain_id', 'date']);
            $table->index(['domain_id', 'date']);
        });
    }

    /**
     * Reverse the migrations.
     */
    public function down(): void
    {
        Schema::dropIfExists('domain_bandwidth_usage');
    }
};
