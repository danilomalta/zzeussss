"use strict";
const path=require('node:path');
function within(root,file,paths=path){const rel=paths.relative(root,file);return rel!==''&&!paths.isAbsolute(rel)&&rel!=='..'&&!rel.startsWith('..'+paths.sep);}
function portableRelative(root,file,paths=path){if(!within(root,file,paths))throw Error('Path escaped build root');const name=paths.relative(root,file).split(paths.sep).join('/');if(/[\\:\x00-\x1f]/.test(name)||name.split('/').some(p=>!p||p==='.'||p==='..'))throw Error('Invalid build path');return name;}
module.exports={within,portableRelative};
