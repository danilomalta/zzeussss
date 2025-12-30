#!/bin/bash

# TitanSystem Initialization Script
# Act as a Senior Chief Technology Officer (CTO)
# This script initializes the entire directory structure for "TitanSystem".

set -e

echo "Initializing TitanSystem..."

# 1. ROOT HIERARCHY
mkdir -p TitanSystem
cd TitanSystem

# ==========================================
# A. BACKEND (titan-backend)
# ==========================================
echo "Setting up titan-backend..."
mkdir -p titan-backend
cd titan-backend

# Initialize Go Module
if [ ! -f go.mod ]; then
    go mod init github.com/titan/backend
fi

# Create pkg/database/sqlite.go
mkdir -p pkg/database
cat <<EOF > pkg/database/sqlite.go
package database

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Connect() (*gorm.DB, error) {
	// SQLite with WAL Mode enabled and busy_timeout=5000
	dsn := "file:titan.db?cache=shared&_journal_mode=WAL&_busy_timeout=5000"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, err
	}
	return db, nil
}
EOF

# Create cmd/api/main.go
mkdir -p cmd/api
cat <<EOF > cmd/api/main.go
package main

func main() {
	// Entrypoint
}
EOF

# Create Internal Modules
MODULES=("auth" "user" "financial" "fiscal" "stock" "sales_pdv" "logistics" "production" "legal" "office" "communication" "marketplace")

for mod in "${MODULES[@]}"; do
    mkdir -p "internal/$mod/domain"
    mkdir -p "internal/$mod/ports"
    mkdir -p "internal/$mod/service"
    mkdir -p "internal/$mod/repository"
done

# Auth specific handlers
mkdir -p internal/auth/handler
cat <<EOF > internal/auth/handler/login_handler.go
package handler
// Login Handler
EOF
cat <<EOF > internal/auth/handler/register_handler.go
package handler
// Register Handler
EOF

cd ..

# ==========================================
# B. WEB & SOFTWARE (titan-web-software)
# ==========================================
echo "Setting up titan-web-software..."
mkdir -p titan-web-software
cd titan-web-software

# Pages
mkdir -p src/pages/auth
cat <<EOF > src/pages/auth/Login.tsx
// Login Page
import React from 'react';
export const Login = () => <div>Login</div>;
EOF
cat <<EOF > src/pages/auth/Register.tsx
// Register Page
import React from 'react';
export const Register = () => <div>Register</div>;
EOF

mkdir -p src/pages/dashboard
cat <<EOF > src/pages/dashboard/Home.tsx
// Home Dashboard
import React from 'react';
export const Home = () => <div>Home</div>;
EOF

# Modules
mkdir -p src/modules
for mod in "${MODULES[@]}"; do
    mkdir -p "src/modules/$mod"
done

# Electron
mkdir -p electron
cat <<EOF > electron/main.js
const { app, BrowserWindow } = require('electron');
// Main Process
EOF

cd ..

# ==========================================
# C. MOBILE (titan-mobile)
# ==========================================
echo "Setting up titan-mobile..."
mkdir -p titan-mobile
cd titan-mobile

# Screens
mkdir -p src/screens/Auth
cat <<EOF > src/screens/Auth/LoginScreen.tsx
// Login Screen
import React from 'react';
import { View, Text } from 'react-native';
export const LoginScreen = () => <View><Text>Login</Text></View>;
EOF
cat <<EOF > src/screens/Auth/RegisterScreen.tsx
// Register Screen
import React from 'react';
import { View, Text } from 'react-native';
export const RegisterScreen = () => <View><Text>Register</Text></View>;
EOF

# Module Screens
mkdir -p src/screens/Modules/Logistics
mkdir -p src/screens/Modules/PDV

cd ..

echo "TitanSystem initialized successfully."
