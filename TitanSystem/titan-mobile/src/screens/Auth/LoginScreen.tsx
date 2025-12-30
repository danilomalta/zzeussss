import React from 'react';
import { View, Text, Button, StyleSheet } from 'react-native';
import apiClient from '../../api/client';

export const LoginScreen = () => {
  const handleLogin = async () => {
    try {
      // Chama o Backend Go
      const response = await apiClient.post('/auth/login', { email: 'admin', pass: '123' });
      console.log("Login Sucesso:", response.data);
    } catch (e) {
      console.error("Erro no Login:", e);
    }
  };

  return (
    <View style={styles.container}>
      <Text style={styles.title}>Titan Mobile</Text>
      <Button title="Entrar no Sistema" onPress={handleLogin} />
    </View>
  );
};

const styles = StyleSheet.create({
  container: { flex: 1, justifyContent: 'center', alignItems: 'center' },
  title: { fontSize: 24, marginBottom: 20, fontWeight: 'bold' }
});
