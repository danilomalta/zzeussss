OCR local da entrega96

Tesseract.js 7.0.0 (Apache-2.0): tesseract.min.js e worker.min.js.
tesseract.js-core 7.x (Apache-2.0): três variantes LSTM WASM; o runtime escolhe uma.
Modelo português: @tesseract.js-data/por 1.0.0, 4.0.0_best_int/por.traineddata.gz.
Pacote de distribuição do modelo: MIT, conforme modelo-package.json.
Origem do modelo: https://github.com/naptha/tessdata.
Fontes: https://github.com/naptha/tesseract.js e https://github.com/naptha/tesseract.js-core.
Licenças incluídas nos arquivos LICENSE desta pasta e de core/.

São aproximadamente 13 MB em disco. Nada desta pasta é carregado antes de o usuário
pedir a leitura. A execução usa um worker e caminhos locais; não usa API de OCR,
CDN nem download de modelo externo. A imagem não é persistida. O texto reconhecido
deve ser revisado pelo operador. É possível preencher os campos manualmente.
