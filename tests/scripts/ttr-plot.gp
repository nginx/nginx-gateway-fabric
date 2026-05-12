set terminal png size 800,600
set title "Scaling resources"
set datafile separator ","
set output outputfile . ""

# X-axis settings
stats inputfile using 1 nooutput
xmax = (STATS_max > 0 ? STATS_max : 1)
set xrange [0:xmax]
set xtics autofreq
set xlabel "# Resources"
set grid xtics

# Y-axis settings
set yrange [0:*]
set ylabel "Time to Ready (s)"
set format y "%.1f"

# Plotting data
plot inputfile using 1:2 with lines lw 2 notitle
