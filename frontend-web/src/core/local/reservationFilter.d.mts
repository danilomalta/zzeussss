import type {Reservations} from './stockRead.mjs';
export type ReservationFilters={order_id?:string;version_id?:string;responsible_id?:string;created_from?:string;created_to?:string;minimum_quantity?:string};
export function reservationFilters(value:ReservationFilters,unit:string):ReservationFilters;
export function filterReservations(page:Reservations,value?:ReservationFilters):{filters:ReservationFilters;items:Reservations['items'];offset:number;total_reservations:number;page_reservations:number;matched_reservations:number;unit:string};
