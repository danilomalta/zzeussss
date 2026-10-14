import { createUniver, LocaleType, mergeLocales, defaultTheme } from '@univerjs/presets';
import { UniverSheetsCorePreset } from '@univerjs/preset-sheets-core';
import '@univerjs/preset-sheets-core/lib/index.css';
import core_pt_BR from '@univerjs/preset-sheets-core/locales/pt-BR';
import core_en_US from '@univerjs/preset-sheets-core/locales/en-US';
import core_es_ES from '@univerjs/preset-sheets-core/locales/es-ES';
import core_fr_FR from '@univerjs/preset-sheets-core/locales/fr-FR';
import core_zh_CN from '@univerjs/preset-sheets-core/locales/zh-CN';
import { UniverSheetsConditionalFormattingPreset } from '@univerjs/preset-sheets-conditional-formatting';
import '@univerjs/preset-sheets-conditional-formatting/lib/index.css';
import conditional_formatting_pt_BR from '@univerjs/preset-sheets-conditional-formatting/locales/pt-BR';
import conditional_formatting_en_US from '@univerjs/preset-sheets-conditional-formatting/locales/en-US';
import conditional_formatting_es_ES from '@univerjs/preset-sheets-conditional-formatting/locales/es-ES';
import conditional_formatting_fr_FR from '@univerjs/preset-sheets-conditional-formatting/locales/fr-FR';
import conditional_formatting_zh_CN from '@univerjs/preset-sheets-conditional-formatting/locales/zh-CN';
import { UniverSheetsDataValidationPreset } from '@univerjs/preset-sheets-data-validation';
import '@univerjs/preset-sheets-data-validation/lib/index.css';
import data_validation_pt_BR from '@univerjs/preset-sheets-data-validation/locales/pt-BR';
import data_validation_en_US from '@univerjs/preset-sheets-data-validation/locales/en-US';
import data_validation_es_ES from '@univerjs/preset-sheets-data-validation/locales/es-ES';
import data_validation_fr_FR from '@univerjs/preset-sheets-data-validation/locales/fr-FR';
import data_validation_zh_CN from '@univerjs/preset-sheets-data-validation/locales/zh-CN';
import { UniverSheetsFilterPreset } from '@univerjs/preset-sheets-filter';
import '@univerjs/preset-sheets-filter/lib/index.css';
import filter_pt_BR from '@univerjs/preset-sheets-filter/locales/pt-BR';
import filter_en_US from '@univerjs/preset-sheets-filter/locales/en-US';
import filter_es_ES from '@univerjs/preset-sheets-filter/locales/es-ES';
import filter_fr_FR from '@univerjs/preset-sheets-filter/locales/fr-FR';
import filter_zh_CN from '@univerjs/preset-sheets-filter/locales/zh-CN';
import { UniverSheetsSortPreset } from '@univerjs/preset-sheets-sort';
import '@univerjs/preset-sheets-sort/lib/index.css';
import sort_pt_BR from '@univerjs/preset-sheets-sort/locales/pt-BR';
import sort_en_US from '@univerjs/preset-sheets-sort/locales/en-US';
import sort_es_ES from '@univerjs/preset-sheets-sort/locales/es-ES';
import sort_fr_FR from '@univerjs/preset-sheets-sort/locales/fr-FR';
import sort_zh_CN from '@univerjs/preset-sheets-sort/locales/zh-CN';
export const sheetLocale = (language: string) =>
  ({
    pt: LocaleType.PT_BR,
    'pt-BR': LocaleType.PT_BR,
    en: LocaleType.EN_US,
    es: LocaleType.ES_ES,
    fr: LocaleType.FR_FR,
    zh: LocaleType.ZH_CN,
    'zh-CN': LocaleType.ZH_CN,
  })[language] || LocaleType.PT_BR;
export function mountSheetEditor(container: HTMLElement, language: string, dark: boolean) {
  return createUniver({
    locale: sheetLocale(language),
    darkMode: dark,
    theme: {
      ...defaultTheme,
      primary: {
        ...defaultTheme.gray,
        50: '#faf8f2',
        100: '#f2e8d6',
        200: '#e5d0ab',
        300: '#ceb58a',
        400: '#b79761',
        500: '#a57b36',
        600: '#8d672c',
        700: '#765a32',
        800: '#56432a',
        900: '#35291b',
      },
      blue: defaultTheme.gray,
      orange: defaultTheme.gray,
      yellow: defaultTheme.gray,
    },
    locales: {
      [LocaleType.PT_BR]: mergeLocales(
        core_pt_BR,
        conditional_formatting_pt_BR,
        data_validation_pt_BR,
        filter_pt_BR,
        sort_pt_BR
      ),
      [LocaleType.EN_US]: mergeLocales(
        core_en_US,
        conditional_formatting_en_US,
        data_validation_en_US,
        filter_en_US,
        sort_en_US
      ),
      [LocaleType.ES_ES]: mergeLocales(
        core_es_ES,
        conditional_formatting_es_ES,
        data_validation_es_ES,
        filter_es_ES,
        sort_es_ES
      ),
      [LocaleType.FR_FR]: mergeLocales(
        core_fr_FR,
        conditional_formatting_fr_FR,
        data_validation_fr_FR,
        filter_fr_FR,
        sort_fr_FR
      ),
      [LocaleType.ZH_CN]: mergeLocales(
        core_zh_CN,
        conditional_formatting_zh_CN,
        data_validation_zh_CN,
        filter_zh_CN,
        sort_zh_CN
      ),
    },
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
