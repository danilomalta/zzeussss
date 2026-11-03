import {mergeLocales} from '@univerjs/presets';
import part0 from '@univerjs/preset-sheets-core/locales/zh-CN';
import part1 from '@univerjs/preset-sheets-conditional-formatting/locales/zh-CN';
import part2 from '@univerjs/preset-sheets-data-validation/locales/zh-CN';
import part3 from '@univerjs/preset-sheets-filter/locales/zh-CN';
import part4 from '@univerjs/preset-sheets-sort/locales/zh-CN';
export default {locale:'zhCN',messages:mergeLocales(part0,part1,part2,part3,part4)};
