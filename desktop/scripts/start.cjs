'use strict';
const {startDesktop}=require('./stage.cjs');
try{process.exitCode=startDesktop(process.argv.slice(2));}catch(error){console.error(error.message);process.exitCode=1;}
