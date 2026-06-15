declare module 'pdfmake' {
    const pdfMake: any;
    export default pdfMake;
}

declare module 'pdfmake/build/pdfmake' {
    const pdfMake: any;
    export default pdfMake;
}

declare module 'pdfmake/build/vfs_fonts' {
    const vfsFonts: {
        pdfMake: { vfs: { [key: string]: string } };
    };
    export default vfsFonts;
}