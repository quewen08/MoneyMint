import { Solar, Lunar, HolidayUtil } from 'lunar-javascript';

/**
 * 获取指定日期的农历日期
 * @param date 可选，指定日期，默认为当前日期
 * @returns 农历日期字符串，格式为"YYYY年MM月DD日"
 */
export const getLunarDate = (date: Date | string = new Date()): string => {
    const solar = Solar.fromDate(new Date(date));
    const lunar = solar.getLunar();
    return lunar.getYearInChinese() + '年' + lunar.getMonthInChinese() + '月' + lunar.getDayInChinese();
};

/**
 * 获取指定日期的农历日期
 * @param date 可选，指定日期，默认为当前日期
 * @returns 农历日期字符串，格式为"D日"
 */
export const getLunarDay = (date: Date | string = new Date()): string => {
    const solar = Solar.fromDate(new Date(date));
    const lunar = solar.getLunar();
    return lunar.getDayInChinese() + '';
};

/**
 * 获取指定日期的节日
 * @param date 可选，指定日期，默认为当前日期
 * @returns 节日名称，如果没有则返回空字符串
 */
export const getFestival = (date: Date | string = new Date()): string => {
    const solar = Solar.fromDate(new Date(date));
    const lunar = solar.getLunar();

    // 优先返回农历节日
    const lunarFestival = lunar.getFestivals();
    if (lunarFestival.length > 0) {
        return lunarFestival[0];
    }

    // 再返回阳历节日
    const solarFestival = solar.getFestivals();
    if (solarFestival.length > 0) {
        return solarFestival[0];
    }

    return "";
};

/**
 * 获取指定日期的农历日期和节日
 * @param date 可选，指定日期，默认为当前日期
 * @returns 农历日期和节日的组合字符串
 */
export const getLunarAndFestival = (date: Date | string = new Date()): string => {
    const lunarDate = getLunarDate(date);
    const festival = getFestival(date);

    if (festival) {
        return `${lunarDate} · ${festival}`;
    }

    return lunarDate;
};