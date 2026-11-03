import {mergeLocales} from '@univerjs/presets';
import part0 from '@univerjs/preset-sheets-core/locales/es-ES';
import part1 from '@univerjs/preset-sheets-conditional-formatting/locales/es-ES';
import part2 from '@univerjs/preset-sheets-data-validation/locales/es-ES';
import part3 from '@univerjs/preset-sheets-filter/locales/es-ES';
import part4 from '@univerjs/preset-sheets-sort/locales/es-ES';
export default {locale:'esES',messages:mergeLocales(part0,part1,part2,part3,part4)};
