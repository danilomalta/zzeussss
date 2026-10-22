import {sheetLanguageKey} from '../../../core/sheets70/language.mjs';
import {loadSheetLanguage} from './sheetLanguage70';
import { createUniver, LocaleType, defaultTheme } from '@univerjs/presets';
import { UniverSheetsCorePreset } from '@univerjs/preset-sheets-core';
import '@univerjs/preset-sheets-core/lib/index.css';
import { UniverSheetsConditionalFormattingPreset } from '@univerjs/preset-sheets-conditional-formatting';
import '@univerjs/preset-sheets-conditional-formatting/lib/index.css';
import { UniverSheetsDataValidationPreset } from '@univerjs/preset-sheets-data-validation';
import '@univerjs/preset-sheets-data-validation/lib/index.css';
import { UniverSheetsFilterPreset } from '@univerjs/preset-sheets-filter';
import '@univerjs/preset-sheets-filter/lib/index.css';
import { UniverSheetsSortPreset } from '@univerjs/preset-sheets-sort';
import '@univerjs/preset-sheets-sort/lib/index.css';
export const sheetLocale = (language:string) => ({ptBR:LocaleType.PT_BR,enUS:LocaleType.EN_US,esES:LocaleType.ES_ES,frFR:LocaleType.FR_FR,zhCN:LocaleType.ZH_CN})[sheetLanguageKey(language)];
export function mountSheetEditor(container: HTMLElement, language: string, dark: boolean, pack: Awaited<ReturnType<typeof loadSheetLanguage>>) {
  return createUniver({
    locale: sheetLocale(language),
    darkMode: dark,
    theme: {
      ...defaultTheme,
      primary: defaultTheme.gray,
      blue: defaultTheme.gray,
      orange: defaultTheme.gray,
      yellow: defaultTheme.gray,
    },
    locales: {[pack.locale]: pack.messages},
    presets: [
      UniverSheetsCorePreset({
        container,
        header: true,
        toolbar: true,
        ribbonType: 'classic',
        footer: { sheetBar: true, statisticBar: true, menus: true, zoomSlider: true },
      }),
      UniverSheetsConditionalFormattingPreset(),
      UniverSheetsDataValidationPreset(),
      UniverSheetsFilterPreset(),
      UniverSheetsSortPreset(),
    ],
  });
}
