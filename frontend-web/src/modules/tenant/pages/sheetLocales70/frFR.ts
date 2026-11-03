import {mergeLocales} from '@univerjs/presets';
import part0 from '@univerjs/preset-sheets-core/locales/fr-FR';
import part1 from '@univerjs/preset-sheets-conditional-formatting/locales/fr-FR';
import part2 from '@univerjs/preset-sheets-data-validation/locales/fr-FR';
import part3 from '@univerjs/preset-sheets-filter/locales/fr-FR';
import part4 from '@univerjs/preset-sheets-sort/locales/fr-FR';
export default {locale:'frFR',messages:mergeLocales(part0,part1,part2,part3,part4)};
