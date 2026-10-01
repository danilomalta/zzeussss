import React from 'react';
import { StyleSheet, Text, View } from 'react-native';

/*
MOBILE APP (REACT NATIVE)
=========================
Planejado, ainda não implementado: aplicativo focado em funções "On-the-go":
1. Aprovação de pagamentos pelo dono.
2. Consulta de estoque em tempo real na loja.
3. [CRÍTICO] Sincronização offline-first: Se o sinal cair dentro do estoque,
   as contagens são salvas no SQLite/WatermelonDB local e sincronizadas com o backend
   Go via WebSockets/REST assim que a conexão retornar.
*/

export default function App() {
  return (
    <View style={styles.container}>
      <Text>TitanSystem Mobile: vendas e sincronização offline ainda não implementadas</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#0f0f0f',
    alignItems: 'center',
    justifyContent: 'center',
  },
});
