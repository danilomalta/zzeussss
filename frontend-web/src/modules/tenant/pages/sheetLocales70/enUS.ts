import {mergeLocales} from '@univerjs/presets';
import part0 from '@univerjs/preset-sheets-core/locales/en-US';
import part1 from '@univerjs/preset-sheets-conditional-formatting/locales/en-US';
import part2 from '@univerjs/preset-sheets-data-validation/locales/en-US';
import part3 from '@univerjs/preset-sheets-filter/locales/en-US';
import part4 from '@univerjs/preset-sheets-sort/locales/en-US';
export default {locale:'enUS',messages:mergeLocales(part0,part1,part2,part3,part4)};
